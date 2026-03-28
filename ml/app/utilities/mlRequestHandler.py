"""
Artifact: KMeans Anomaly Detection Service
Description: Runs KMeans anomaly detection on TempSample data and can serve results over HTTP for the ingest pipeline
Authors: Jacob Kice, Minh Vu, Adam Berry
Date Created: 03/28/2026
Date Revised: 03/28/2026
Preconditions: Input data is valid json with temperature sample fields and sklearn dependencies are installed
Postconditions: Returns anomaly results either to stdout in CLI mode or as json in API mode
Possible errors: FileNotFoundError, JSONDecodeError, KeyError for missing fields, ValueError for bad request payloads
Side effects: Starts an HTTP server in API mode
Invariants: KMeans logic stays based on fan_on, temperature, and heater_power
Known faults: Small sample sets can reduce cluster count to avoid fit failures
"""

import json
import os
from http.server import BaseHTTPRequestHandler
import logging
import matplotlib
import matplotlib.pyplot as plt
from utilities.checkpointing import (
    checkpoint_manager,
    prepare_temperature_features,
    refresh_temperature_baseline_model,
    temperature_model_is_ready,
    update_recent_history,
    build_temperature_baseline_model
)

import pandas as pd
import numpy as np
from sklearn.decomposition import PCA

logger = logging.getLogger(__name__)

def dataframe_from_request(data):
    """
    Converts either the raw sample list or the ingest request body into a DataFrame.
    Args:
            data: Either a list of sample dicts or an object with event_type and samples fields.
    Returns:
            Tuple of event_type and DataFrame built from the provided samples.
    """
    if isinstance(data, list):
        return "temperature", pd.DataFrame(data)
    if not isinstance(data, dict):
        logger.error("Request body must be an object with event_type and sample")
        raise ValueError("Request body must be an object with event_type and samples")
    event_type = data.get("event_type")
    if not isinstance(event_type, str) or not event_type.strip():
        logger.error("Request body must include event_type")
        raise ValueError("Request body must include event_type")
    samples = data.get("samples")
    if not isinstance(samples, list):
        logger.error("Request body must include samples as a list")
        raise ValueError("Request body must include samples as a list")
    return event_type, pd.DataFrame(samples)


def kmeans_anomaly_detection(df, model_state=None, n_clusters=3, outlier_percentile=95):
    """
    Runs KMeans anomaly scoring and flags outliers based on distance to a baseline model.
    Args:
            df: DataFrame of TempSample data
            model_state: Optional persisted baseline model to score against.
            n_clusters: Number of clusters for KMeans
            outlier_percentile: Percentile threshold for outlier detection
    Returns:
            DataFrame with detected_anomaly and anomaly_score columns
    """
    if df.empty:
        return df.copy()

    features = prepare_temperature_features(df)

    # Very small or low-variety batches are not meaningful for clustering and
    # tend to mark nearly every batch as anomalous. Treat them as "no anomaly".
    if len(features) < 5 and model_state is None:
        df = df.copy()
        df["detected_anomaly"] = 0
        df["anomaly_score"] = 0.0
        df["anomaly_label"] = ""
        return df

    unique_count = len(features.drop_duplicates())
    if unique_count < 3 and model_state is None:
        df = df.copy()
        df["detected_anomaly"] = 0
        df["anomaly_score"] = 0.0
        df["anomaly_label"] = ""
        return df

    #  If the model isn't ready then
    if not temperature_model_is_ready(model_state):
        logger.info("temperature model isn't ready")
        model_state = build_temperature_baseline_model(
            df,
            n_clusters=n_clusters,
            outlier_percentile=outlier_percentile,
        )
        if model_state is None:
            df = df.copy()
            df["detected_anomaly"] = 0
            df["anomaly_score"] = 0.0
            df["anomaly_label"] = ""
            return df

    scaled_features = model_state["scaler"].transform(features)
    cluster_labels = model_state["kmeans"].predict(scaled_features)
    distances = model_state["kmeans"].transform(scaled_features).min(axis=1)

    threshold = float(
        model_state.get("threshold", np.percentile(distances, outlier_percentile))
    )

    # Mark as outlier if distance is at or above threshold
    df = df.copy()
    df["cluster_label"] = cluster_labels
    df["detected_anomaly"] = (distances > threshold).astype(int)
    df["anomaly_score"] = distances
    df["anomaly_label"] = np.where(df["detected_anomaly"] == 1, "kmeans_outlier", "")
    return df


def valve_anomaly_detection(df):
    """
    Runs a simple rule-based anomaly detector for ValveSample data.
    Args:
            df: DataFrame of ValveSample data
    Returns:
            DataFrame with detected_anomaly and anomaly_score columns
    """
    if df.empty:
        return df.copy()

    flow_rate = pd.to_numeric(df.get("flow_rate", 0.0), errors="coerce").fillna(0.0)
    is_open = df.get("is_open", False)
    if not isinstance(is_open, pd.Series):
        is_open = pd.Series([bool(is_open)] * len(df))
    is_open = is_open.astype(bool)

    # Flag impossible or suspicious flow states
    flow_when_closed = (~is_open) & (flow_rate > 0.0)
    bad_open_flow = is_open & ((flow_rate < 1.0) | (flow_rate > 250.0))
    flags = flow_when_closed | bad_open_flow

    labels = []
    for closed_flow, open_flow in zip(flow_when_closed, bad_open_flow):
        if closed_flow:
            labels.append("valve_flow_while_closed")
        elif open_flow:
            labels.append("valve_flow_out_of_range")
        else:
            labels.append("")

    df = df.copy()
    df["detected_anomaly"] = flags.astype(int)
    df["anomaly_score"] = flow_rate.abs()
    df["anomaly_label"] = labels
    return df


def build_anomaly_response(result_df, event_type, model):
    """
    Builds a stable json response for the ingest service.
    Args:
            result_df: DataFrame produced by one of the anomaly detection helpers
            event_type: Event type associated with the analyzed batch
            model: Model identifier for the response
    Returns:
            Dictionary with top level anomaly summary and per sample details.
    """
    if result_df.empty:
        return {
            "model": model,
            "event_type": event_type,
            "has_anomaly": False,
            "label": "",
            "labels": [],
            "score": None,
            "sample_count": 0,
            "anomaly_count": 0,
            "anomalies": [],
        }

    anomalies = []
    for index, row in result_df.iterrows():
        is_anomaly = bool(row.get("detected_anomaly", 0))
        label = row.get("anomaly_label", "") if is_anomaly else ""
        score = float(row.get("anomaly_score", 0.0))
        entry = {
            "index": int(index),
            "id": int(row["id"]) if "id" in row and pd.notna(row["id"]) else None,
            "timestamp": row.get("timestamp"),
            "cluster_label": (
                int(row["cluster_label"])
                if "cluster_label" in row and pd.notna(row["cluster_label"])
                else None
            ),
            "is_anomaly": is_anomaly,
            "label": label,
            "score": score,
        }
        if is_anomaly:
            anomalies.append(entry)

    labels = sorted({entry["label"] for entry in anomalies if entry["label"]})
    max_score = (
        float(result_df["anomaly_score"].max())
        if "anomaly_score" in result_df
        else None
    )
    has_anomaly = len(anomalies) > 0
    return {
        "model": model,
        "event_type": event_type,
        "has_anomaly": has_anomaly,
        "label": labels[0] if labels else "",
        "labels": labels,
        "score": max_score,
        "sample_count": int(len(result_df)),
        "anomaly_count": int(len(anomalies)),
        "anomalies": anomalies,
    }


def analyze_temperature_batch(df, n_clusters=3, outlier_percentile=95):
    """
    Run temperature anomaly detection and return both the detailed frame and
    the stable API response.
    Args:
            df: DataFrame of temperature samples.
            n_clusters: Number of clusters for KMeans.
            outlier_percentile: Outlier percentile threshold.
    Returns:
            Tuple of (result_df, response_dict, model_state).
    """
    required = {"fan_on", "temperature", "heater_power"}
    missing = sorted(required.difference(df.columns))
    if missing:
        logger.error("temperature samples missing fields: %s", missing)
        raise ValueError(f'temperature samples missing fields: {", ".join(missing)}')

    model_state = checkpoint_manager.models.get("temperature_kmeans")
    if not temperature_model_is_ready(model_state):
        refresh_temperature_baseline_model(
            n_clusters=n_clusters, outlier_percentile=outlier_percentile
        )
        model_state = checkpoint_manager.models.get("temperature_kmeans")

    result = kmeans_anomaly_detection(
        df,
        model_state=model_state,
        n_clusters=n_clusters,
        outlier_percentile=outlier_percentile,
    )
    update_recent_history("temperature", df)
    refresh_temperature_baseline_model(
        n_clusters=n_clusters, outlier_percentile=outlier_percentile
    )
    response = build_anomaly_response(result, "temperature", "kmeans")
    return result, response, model_state


def save_temperature_visualization(result_df, output_path, title=None):
    """
    Save a 2D PCA projection of the analyzed temperature batch.
    Args:
            result_df: DataFrame returned by kmeans_anomaly_detection.
            output_path: File path to write the plot image to.
            title: Optional chart title.
    Returns:
            Absolute path to the saved image.
    """
    if result_df.empty:
        logger.error("Tried to save an empty temperature batch")
        raise ValueError("cannot visualize an empty temperature batch")

    features = prepare_temperature_features(result_df)
    if len(features) < 2:
        logger.error("Less than two samples")
        raise ValueError(
            "at least two temperature samples are required to visualize clusters"
        )

    try:

        matplotlib.use("Agg")
    except ImportError as err:
        raise RuntimeError(
            "matplotlib is required to generate visualization output"
        ) from err

    reduced = PCA(n_components=2).fit_transform(features)
    plot_df = result_df.copy()
    plot_df["pca_x"] = reduced[:, 0]
    plot_df["pca_y"] = reduced[:, 1]

    cluster_labels = (
        sorted(plot_df["cluster_label"].dropna().astype(int).unique())
        if "cluster_label" in plot_df
        else []
    )
    plt.figure(figsize=(8, 5))

    if cluster_labels:
        for cluster_label in cluster_labels:
            cluster_points = plot_df[plot_df["cluster_label"] == cluster_label]
            normal_points = cluster_points[cluster_points["detected_anomaly"] == 0]
            if not normal_points.empty:
                plt.scatter(
                    normal_points["pca_x"],
                    normal_points["pca_y"],
                    label=f"Cluster {cluster_label}",
                    s=65,
                    alpha=0.8,
                )
    else:
        normal_points = plot_df[plot_df["detected_anomaly"] == 0]
        if not normal_points.empty:
            plt.scatter(
                normal_points["pca_x"],
                normal_points["pca_y"],
                label="Normal samples",
                s=65,
                alpha=0.8,
                color="tab:blue",
            )

    anomaly_points = plot_df[plot_df["detected_anomaly"] == 1]
    if not anomaly_points.empty:
        plt.scatter(
            anomaly_points["pca_x"],
            anomaly_points["pca_y"],
            label="Anomalies",
            s=150,
            marker="x",
            linewidths=2.0,
            color="red",
        )

    plt.title(title or "Temperature Clusters and Anomalies")
    plt.xlabel("PCA 1")
    plt.ylabel("PCA 2")
    plt.legend()
    plt.tight_layout()

    output_dir = os.path.dirname(os.path.abspath(output_path))
    if output_dir:
        os.makedirs(output_dir, exist_ok=True)
    plt.savefig(output_path, dpi=150)
    plt.close()
    return os.path.abspath(output_path)


def analyze_temperature(df, n_clusters=3, outlier_percentile=95):
    """
    Runs the end to end temperature analysis path.
    Args:
            df: DataFrame of temperature samples.
            n_clusters: Number of clusters for KMeans.
            outlier_percentile: Outlier percentile threshold.
    Returns:
            Stable anomaly response dictionary.
    """
    _, response, _ = analyze_temperature_batch(
        df,
        n_clusters=n_clusters,
        outlier_percentile=outlier_percentile,
    )
    return response


def analyze_valve(df):
    """
    Runs the end to end valve analysis path.
    Args:
            df: DataFrame of valve samples.
    Returns:
            Stable anomaly response dictionary.
    """
    required = {"is_open", "flow_rate"}
    missing = sorted(required.difference(df.columns))
    if missing:
        raise ValueError(f'valve samples missing fields: {", ".join(missing)}')
    result = valve_anomaly_detection(df)
    return build_anomaly_response(result, "valve", "rule_engine")


def analyze_samples(data, n_clusters=3, outlier_percentile=95):
    """
    Runs the end to end analysis path for either CLI or API usage.
    Args:
            data: Raw sample list or request body object.
            n_clusters: Number of clusters for KMeans.
            outlier_percentile: Outlier percentile threshold.
    Returns:
            Stable anomaly response dictionary.
    """
    event_type, df = dataframe_from_request(data)
    if event_type == "temperature":
        return analyze_temperature(
            df, n_clusters=n_clusters, outlier_percentile=outlier_percentile
        )
    if event_type == "valve":
        return analyze_valve(df)

    logger.error("unsupported event type: %s", event_type)
    raise ValueError(f"unsupported event_type: {event_type}")


class MLRequestHandler(BaseHTTPRequestHandler):
    """
    Simple HTTP handler that exposes the anomaly detection over POST.
    """

    clusters = 3
    percentile = 95

    # Handles incoming POST requests
    def do_POST(self):
        # Accept only / and /analyze routes
        if self.path not in ("/", "/analyze"):
            logger.warning("Path not found: %s", self.path)
            self.write_json({"error": "not_found"}, 404)
            return

        # Read request body
        content_length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(content_length) if content_length > 0 else b""
        try:
            # Analyze payload
            payload = json.loads(body or b"{}")
            response = analyze_samples(
                payload, n_clusters=self.clusters, outlier_percentile=self.percentile
            )
        except (json.JSONDecodeError, ValueError, KeyError) as err:
            logger.warning("failed to decode JSON: %s", err)
            self.write_json({"error": str(err)}, 400)
            return
        except Exception as err:
            logger.warning("Exception in ML request handling: %s", err)
            self.write_json({"error": str(err)}, 500)
            return

        # Return JSON back to client
        self.write_json(response, 200)

    # Handles incoming GET requests (Health checks mainly for down the line)
    def do_GET(self):
        if self.path not in ("/", "/health"):
            self.write_json({"error": "not_found"}, 404)
            return
        self.write_json({"status": "ok", "model": "multi_type"}, 200)

    # Just overwritten the default server access logs so the console isn't spammed
    def log_message(self, format, *args):
        return

    # Helper function that writes a JSON response with status code / headers
    def write_json(self, payload, status_code):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status_code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

