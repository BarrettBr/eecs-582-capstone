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
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import GridSearchCV
from sklearn.preprocessing import LabelEncoder

logger = logging.getLogger(__name__)

RF_TRAINING_DATA_PATH = os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "test_samples_suggestions.json")
)
_rf_model = None
_rf_label_encoder = None
_rf_model_ready = False


def _load_rf_training_data(path):
    with open(path, "r", encoding="utf-8") as fp:
        data = json.load(fp)
    return pd.DataFrame(data)


def _prepare_rf_features_and_target(df):
    features = df.loc[:, ["fan_on", "temperature", "heater_power"]].copy()
    features["fan_on"] = features["fan_on"].astype(int)

    missing = [field for field in ["action_class", "priority", "recommended_actions"] if field not in df]
    if missing:
        raise ValueError(f"RF training data missing fields: {', '.join(missing)}")

    action_class = df.loc[:, "action_class"].astype(str)
    priority = df.loc[:, "priority"].astype(str)
    recommended_actions = df.loc[:, "recommended_actions"].apply(
        lambda x: ",".join(x) if isinstance(x, list) and len(x) > 0 else "none"
    )
    target = action_class + "|" + priority + "|" + recommended_actions

    le = LabelEncoder()
    target_encoded = le.fit_transform(target)
    return features, target_encoded, le


def _initialize_rf_model():
    global _rf_model, _rf_label_encoder, _rf_model_ready
    if _rf_model_ready:
        return

    try:
        df = _load_rf_training_data(RF_TRAINING_DATA_PATH)
        X, y, le = _prepare_rf_features_and_target(df)
        if len(X) < 10 or len(set(y)) < 2:
            raise ValueError("RF training data is too small or has too few classes")

        param_grid = {
            "n_estimators": [10, 50],
            "max_depth": [3, 5, None],
            "min_samples_split": [2, 5],
        }
        grid = GridSearchCV(
            RandomForestClassifier(random_state=42),
            param_grid,
            cv=3,
            scoring="accuracy",
            n_jobs=1,
        )
        grid.fit(X, y)
        _rf_model = grid.best_estimator_
        _rf_label_encoder = le
        _rf_model_ready = True
        logger.info("Random Forest model initialized with params: %s", grid.best_params_)
    except Exception as err:
        logger.warning("Random Forest initialization failed: %s", err)
        _rf_model = None
        _rf_label_encoder = None
        _rf_model_ready = False


def _rf_response_for_temperature(df):
    if not _rf_model_ready:
        _initialize_rf_model()
    if not _rf_model_ready or _rf_model is None or _rf_label_encoder is None:
        return None

    if df.empty:
        return None

    missing = sorted({"fan_on", "temperature", "heater_power"}.difference(df.columns))
    if missing:
        return None

    features = df.loc[:, ["fan_on", "temperature", "heater_power"]].copy()
    features["fan_on"] = features["fan_on"].astype(int)

    try:
        proba = _rf_model.predict_proba(features)
        preds = _rf_model.predict(features)
    except Exception as err:
        logger.warning("Random Forest prediction failed: %s", err)
        return None

    pred_labels = _rf_label_encoder.inverse_transform(preds)
    scores = np.max(proba, axis=1).astype(float)
    most_likely_label = pred_labels[0] if len(pred_labels) > 0 else ""
    most_likely_score = float(scores[0]) if len(scores) > 0 else 0.0

    if "|" in most_likely_label:
        parts = most_likely_label.split("|", 2)
        action_class = parts[0] if len(parts) > 0 else ""
        priority = parts[1] if len(parts) > 1 else "low"
        recommended_actions = parts[2] if len(parts) > 2 else ""
    else:
        action_class = most_likely_label
        priority = "low"
        recommended_actions = ""

    severity_map = {
        "high": "high",
        "medium": "medium",
        "low": "low",
    }
    severity = severity_map.get(priority.lower(), "low")
    has_anomaly = action_class != "no_action" or severity in {"high", "medium"}
    probable_cause = (
        f"Random forest predicted {action_class} with priority {priority}."
        if action_class
        else "Random forest did not identify a clear action."
    )
    recommended_action = (
        recommended_actions.replace(",", ", ") if recommended_actions else "Review predicted action class."
    )

    return {
        "model": "random_forest",
        "event_type": "temperature",
        "has_anomaly": has_anomaly,
        "severity": severity,
        "confidence": float(np.mean(scores)) if len(scores) > 0 else 0.0,
        "probable_cause": probable_cause,
        "recommended_action": recommended_action,
        "labels": sorted(set(pred_labels.tolist())),
        "score": float(np.mean(scores)) if len(scores) > 0 else 0.0,
        "sample_count": int(len(features)),
        "anomaly_count": int(np.sum(pred_labels != pred_labels[0])),
        "anomalies": [
            {
                "index": int(i),
                "label": pred_labels[i],
                "score": float(scores[i]),
                "is_anomaly": pred_labels[i] != pred_labels[0],
            }
            for i in range(len(pred_labels))
        ],
    }


def infer_response_severity(score, anomaly_count, sample_count):
    """
    Classify anomaly severity from the current score and batch concentration.
    Args:
            score: Maximum anomaly score for the batch, or None.
            anomaly_count: Number of anomalous samples in the batch.
            sample_count: Total number of samples in the batch.
    Returns:
            Severity string for downstream UI and report consumers.
    """
    if anomaly_count <= 0:
        return "info"

    ratio = anomaly_count / max(1, sample_count)
    if score is None:
        if ratio >= 0.5:
            return "high"
        elif ratio >= 0.2:
            return "medium"
        else:
            return "low"
    else:
        if score >= 3.0 or ratio >= 0.5:
            return "high"
        elif score >= 1.5 or ratio >= 0.2:
            return "medium"
        else:
            return "low"


def infer_confidence(score, has_anomaly):
    """
    Convert the current score into a bounded confidence-like value.
    Args:
            score: Maximum anomaly score for the batch, or None.
            has_anomaly: Whether the batch contains at least one anomaly.
    Returns:
            Confidence float in the range [0, 1].
    """
    if score is None:
        return 0.85 if has_anomaly else 0.35
    # normalize score between 0 and 1
    return float(max(0.0, min(1.0, score / 3.0)))


def infer_probable_cause(event_type, labels):
    """
    Map ML labels into a short human-readable probable cause.
    Args:
            event_type: Event type associated with the batch.
            labels: Deduplicated anomaly labels.
    Returns:
            Short probable cause text.
    """
    label_set = set(labels)
    if "kmeans_outlier" in label_set:
        return "Temperature behavior deviated from the recent operating range."
    if "valve_flow_while_closed" in label_set:
        return "Valve flow was detected while the valve state was closed."
    if "valve_flow_out_of_range" in label_set:
        return "Valve flow was outside the expected range for the open state."
    if event_type == "temperature":
        return "Temperature readings fell outside the recent cluster baseline."
    if event_type == "valve":
        return "Valve telemetry did not match the expected operating state."
    return "Observed behavior deviated from the recent baseline."


def infer_recommended_action(event_type, labels, severity):
    """
    Map ML labels into a short operator-facing recommendation.
    Args:
            event_type: Event type associated with the batch.
            labels: Deduplicated anomaly labels.
            severity: Severity bucket for the current anomaly response.
    Returns:
            Short recommended action text.
    """
    label_set = set(labels)
    if "valve_flow_while_closed" in label_set:
        return "Inspect the valve actuator and confirm the closed-state signal matches physical flow."
    if "valve_flow_out_of_range" in label_set:
        return "Inspect the valve and downstream line for blockage, leakage, or misreported open state."
    if event_type == "temperature":
        if severity == "high":
            return "Inspect the heater, fan, and sensor path immediately for unstable or unsafe operation."
        return "Review recent heater and fan behavior and confirm the sensor remains calibrated."
    if event_type == "valve":
        return "Review valve state transitions and verify the flow sensor reading is plausible."
    return "Review the affected service and compare the latest readings against recent normal behavior."

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
        logger.info("received empty dataframe")
        return df.copy()

    features = prepare_temperature_features(df)

    # Very small or low-variety batches are not meaningful for clustering and
    # tend to mark nearly every batch as anomalous. Treat them as "no anomaly".
    if len(features) < 5 and model_state is None:
        logger.info("small batch, treated as no anomaly")
        df = df.copy()
        df["detected_anomaly"] = 0
        df["anomaly_score"] = 0.0
        df["anomaly_label"] = ""
        return df

    unique_count = len(features.drop_duplicates())
    if unique_count < 3 and model_state is None:
        logger.info("small batch, treated as no anomaly")
        df = df.copy()
        df["detected_anomaly"] = 0
        df["anomaly_score"] = 0.0
        df["anomaly_label"] = ""
        return df

    #  If the model isn't ready then
    if not temperature_model_is_ready(model_state):
        logger.info("building temperature model")
        model_state = build_temperature_baseline_model(
            df,
            n_clusters=n_clusters,
            outlier_percentile=outlier_percentile,
        )
        if model_state is None:
            logger.info("No model, treated as no anomaly")
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
            "severity": "info",
            "confidence": 0.0,
            "probable_cause": "",
            "recommended_action": "",
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
    severity = infer_response_severity(max_score, len(anomalies), len(result_df))
    confidence = infer_confidence(max_score, has_anomaly)
    return {
        "model": model,
        "event_type": event_type,
        "has_anomaly": has_anomaly,
        "severity": severity,
        "confidence": confidence,
        "probable_cause": infer_probable_cause(event_type, labels),
        "recommended_action": infer_recommended_action(event_type, labels, severity),
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
    the stable API responses for KMeans and Random Forest.
    Args:
            df: DataFrame of temperature samples.
            n_clusters: Number of clusters for KMeans.
            outlier_percentile: Outlier percentile threshold.
    Returns:
            Tuple of (kmeans_result_df, kmeans_response_dict, model_state, rf_response_dict).
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

    kmeans_result = kmeans_anomaly_detection(
        df,
        model_state=model_state,
        n_clusters=n_clusters,
        outlier_percentile=outlier_percentile,
    )
    update_recent_history("temperature", df)
    refresh_temperature_baseline_model(
        n_clusters=n_clusters, outlier_percentile=outlier_percentile
    )
    kmeans_response = build_anomaly_response(kmeans_result, "temperature", "kmeans")
    rf_response = _rf_response_for_temperature(df)
    return kmeans_result, kmeans_response, model_state, rf_response


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
        logger.error("Less than two samples for visualization")
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
            List of stable anomaly response dictionaries for each temperature model.
    """
    _, kmeans_response, _, rf_response = analyze_temperature_batch(
        df,
        n_clusters=n_clusters,
        outlier_percentile=outlier_percentile,
    )
    responses = [kmeans_response]
    if rf_response is not None:
        responses.append(rf_response)
    return responses


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
        logger.warning("Valve data is missing a field")
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
            Stable anomaly response dictionary or list of response dictionaries.
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
    Simple HTTP handler that exposes the anomaly detection over HTTP.
    """

    clusters = 3
    percentile = 95

    def do_POST(self):
        """ Handles incoming POST requests from the ingest side"""
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
