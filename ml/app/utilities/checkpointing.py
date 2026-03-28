"""
Artifact: ML checkpointing service
Description: Persists ML checkpoint state, maintains retained warmup history, and rebuilds the reusable temperature baseline model across process restarts
Authors: Minh Vu, Adam Berry
Date Created: ~, file created 03/28/2026
Date Revised: 03/28/2026
Preconditions: callers provide valid SQLite paths or checkpoint files when using warmup and restore flows
Postconditions: Checkpoints can be saved and restored, recent sample history is retained in memory, and the temperature baseline model can be rebuilt from retained history
Possible errors: File I/O failures, pickle deserialization failures, SQLite access/query failures, and invalid or insufficient retained data for baseline model construction
Side effects: Creates the checkpoints directory if needed, reads and writes checkpoint files, loads historical samples from SQLite, and mutates module-level checkpoint and history state
Invariants: Checkpoint state remains keyed by model name, retained history stays bounded per event type, and restored temperature models must include scaler, clusterer, and threshold data
Known faults: Warmup and restore paths depend on schema and serialized model compatibility, and very small or low-variety temperature history cannot produce a baseline model
"""
import os
import pickle
import sqlite3
import time
from datetime import datetime
import logging

import numpy as np
import pandas as pd
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler


logger = logging.getLogger(__name__)

# Global checkpoint manager
APP_DIR = os.path.dirname(os.path.abspath(__file__))
CHECKPOINT_DIR = os.path.normpath(os.path.join(APP_DIR, "..", "checkpoints"))
os.makedirs(CHECKPOINT_DIR, exist_ok=True)

TEMPERATURE_HISTORY_LIMIT = 512
VALVE_HISTORY_LIMIT = 128
HISTORY_LIMITS = {
    "temperature": TEMPERATURE_HISTORY_LIMIT,
    "valve": VALVE_HISTORY_LIMIT,
}
RECENT_HISTORY = {
    "temperature": pd.DataFrame(),
    "valve": pd.DataFrame(),
}


def prepare_temperature_features(df):
    """
    Extract the temperature feature columns used by the clustering model.
    Args:
            df: Temperature sample frame.
    Returns:
            DataFrame with normalized feature dtypes ready for scaling.
    """
    features = df.loc[:, ["fan_on", "temperature", "heater_power"]].copy()
    features["fan_on"] = features["fan_on"].astype(int)
    return features


def build_temperature_baseline_model(df, n_clusters=3, outlier_percentile=95):
    """
    Fit a reusable temperature baseline model from historical samples only.
    Args:
            df: Historical temperature samples.
            n_clusters: Number of KMeans clusters to fit.
            outlier_percentile: Percentile threshold derived from baseline distances.
    Returns:
            Dictionary describing the fitted baseline, or None if insufficient data.
    """
    if df.empty:
        return None

    features = prepare_temperature_features(df)
    if len(features) < 5:
        return None

    unique_count = len(features.drop_duplicates())
    if unique_count < 3:
        return None

    cluster_count = min(max(1, len(features)), n_clusters, unique_count)
    scaler = StandardScaler()
    scaled_features = scaler.fit_transform(features)
    kmeans = KMeans(n_clusters=cluster_count, random_state=42)
    kmeans.fit(scaled_features)
    distances = kmeans.transform(scaled_features).min(axis=1)
    threshold = float(np.percentile(distances, outlier_percentile))
    return {
        "kmeans": kmeans,
        "scaler": scaler,
        "threshold": threshold,
        "n_clusters": cluster_count,
        "baseline_samples": int(len(features)),
        "outlier_percentile": float(outlier_percentile),
        "fitted_at": datetime.now().isoformat(),
    }


def refresh_temperature_baseline_model(n_clusters=3, outlier_percentile=95):
    """
    Rebuild the persisted temperature baseline model from retained history.
    Args:
            n_clusters: Requested cluster count.
            outlier_percentile: Percentile used to derive the anomaly threshold.
    Returns:
            True when a baseline model is available after refresh.
    """
    history = RECENT_HISTORY.get("temperature", pd.DataFrame())
    model = build_temperature_baseline_model(
        history,
        n_clusters=n_clusters,
        outlier_percentile=outlier_percentile,
    )
    checkpoint_manager.models["temperature_kmeans"] = model
    return model is not None


def temperature_model_is_ready(model_state):
    """
    Validate that a restored temperature checkpoint contains the pieces needed
    for scoring.
    Args:
            model_state: Persisted model payload.
    Returns:
            True when the model can score new samples.
    """
    required_keys = {"kmeans", "scaler", "threshold"}
    return isinstance(model_state, dict) and required_keys.issubset(model_state.keys())


def update_recent_history(event_type, df):
    """
    Stores a rolling history window so unsupervised detection can compare a small
    current batch against recent baseline behavior.
    Args:
            event_type: Event family for the provided samples.
            df: Current batch DataFrame.
    Returns:
            None.
    """
    if event_type not in RECENT_HISTORY:
        RECENT_HISTORY[event_type] = pd.DataFrame()
    combined = pd.concat([RECENT_HISTORY[event_type], df.copy()], ignore_index=True)
    history_limit = HISTORY_LIMITS.get(event_type, TEMPERATURE_HISTORY_LIMIT)
    RECENT_HISTORY[event_type] = combined.tail(history_limit).reset_index(drop=True)


class ModelCheckpoint:
    """
    Manages saving and loading model checkpoints for warmup training.
    """

    def __init__(self, checkpoint_dir=CHECKPOINT_DIR):
        self.checkpoint_dir = checkpoint_dir
        self.models = {"temperature_kmeans": None, "valve_model": None}
        self.training_state = {
            "epoch": 0,
            "samples_processed": 0,
            "last_checkpoint_time": None,
            "history_size": {"temperature": 0, "valve": 0},
        }

    def save_checkpoint(self, checkpoint_name=None):
        """
        Save current model state and training progress.
        """
        if checkpoint_name is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            checkpoint_name = f"checkpoint_{timestamp}"

        checkpoint_path = os.path.join(self.checkpoint_dir, f"{checkpoint_name}.pkl")

        checkpoint_data = {
            "models": self.models,
            "training_state": self.training_state,
            "recent_history": RECENT_HISTORY.copy(),
            "saved_at": datetime.now().isoformat(),
        }

        try:
            with open(checkpoint_path, "wb") as f:
                pickle.dump(checkpoint_data, f)
            logger.info("Checkpoint saved: %s", checkpoint_path)
            return checkpoint_path
        except Exception as e:
            logger.warning("Failed to save checkpoint: %s", e)
            return None

    def load_checkpoint(self, checkpoint_path):
        """
        Load model state and training progress from checkpoint.
        """
        if not os.path.exists(checkpoint_path):
            logger.warning("Checkpoint not found: %s", checkpoint_path)
            return False

        try:
            with open(checkpoint_path, "rb") as f:
                checkpoint_data = pickle.load(f)

            self.models = checkpoint_data.get("models", self.models)
            self.training_state = checkpoint_data.get(
                "training_state", self.training_state
            )

            # Restore recent history
            global RECENT_HISTORY
            RECENT_HISTORY = checkpoint_data.get("recent_history", RECENT_HISTORY)

            saved_at = checkpoint_data.get("saved_at", "unknown")
            logger.warning("Checkpoint loaded: %s (saved at %s)", checkpoint_path, saved_at)
            return True
        except Exception as e:
            logger.warning("Failed to load checkpoint: %s", e)
            return False

    def get_latest_checkpoint(self):
        """
        Get the most recent checkpoint file.
        """
        if not os.path.exists(self.checkpoint_dir):
            return None

        checkpoint_files = [
            f for f in os.listdir(self.checkpoint_dir) if f.endswith(".pkl")
        ]
        if not checkpoint_files:
            return None

        # Sort by modification time (newest first)
        checkpoint_files.sort(
            key=lambda x: os.path.getmtime(os.path.join(self.checkpoint_dir, x)),
            reverse=True,
        )
        return os.path.join(self.checkpoint_dir, checkpoint_files[0])

    def update_training_state(self, samples_processed=0):
        """
        Update training progress counters.
        """
        self.training_state["samples_processed"] += samples_processed
        self.training_state["last_checkpoint_time"] = datetime.now().isoformat()
        self.training_state["history_size"] = {
            "temperature": len(RECENT_HISTORY.get("temperature", pd.DataFrame())),
            "valve": len(RECENT_HISTORY.get("valve", pd.DataFrame())),
        }

    def get_status(self):
        """
        Get current training status.
        """
        return {
            "epoch": self.training_state["epoch"],
            "samples_processed": self.training_state["samples_processed"],
            "history_sizes": self.training_state["history_size"],
            "last_checkpoint": self.training_state["last_checkpoint_time"],
            "models_loaded": {k: v is not None for k, v in self.models.items()},
        }


checkpoint_manager = ModelCheckpoint()




def warmup_from_database(db_path, limit=32, n_clusters=3, outlier_percentile=95):
    """
    Extracts historical data from the ingest service's SQLite database for model warmup.
    Args:
            db_path: Path to the SQLite database file.
            limit: Maximum number of samples to extract per event type.
    Returns:
            None. Updates RECENT_HISTORY in place.
    """
    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        # Extract temperature samples in chronological order so retained history
        # mirrors the live stream ordering.
        cursor.execute(
            """
			SELECT id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
			FROM temp_samples
			ORDER BY timestamp ASC
			LIMIT ?
		""",
            (limit,),
        )

        temp_rows = cursor.fetchall()
        if temp_rows:
            temp_samples = []
            for row in temp_rows:
                temp_samples.append(
                    {
                        "id": row[0],
                        "timestamp": row[1],
                        "sensor_type": row[2],
                        "sensor_number": row[3],
                        "fan_on": bool(row[4]),
                        "temperature": row[5],
                        "heater_power": row[6],
                        "anomalies": [],
                    }
                )
            temp_df = pd.DataFrame(temp_samples)
            update_recent_history("temperature", temp_df)
            checkpoint_manager.update_training_state(len(temp_df))
            refresh_temperature_baseline_model(
                n_clusters=n_clusters,
                outlier_percentile=outlier_percentile,
            )
            logger.info("Loaded %s temperature samples from database", len(temp_df))

        # Extract valve samples in chronological order for consistency.
        cursor.execute(
            """
			SELECT id, timestamp, sensor_type, valve_number, is_open, flow_rate
			FROM valve_samples
			ORDER BY timestamp ASC
			LIMIT ?
		""",
            (limit,),
        )

        valve_rows = cursor.fetchall()
        if valve_rows:
            valve_samples = []
            for row in valve_rows:
                valve_samples.append(
                    {
                        "id": row[0],
                        "timestamp": row[1],
                        "sensor_type": row[2],
                        "sensor_number": row[3],
                        "is_open": bool(row[4]),
                        "flow_rate": row[5],
                        "anomalies": [],
                    }
                )
            valve_df = pd.DataFrame(valve_samples)
            update_recent_history("valve", valve_df)
            checkpoint_manager.update_training_state(len(valve_df))
            logger.info("Loaded %s valve samples from database", len(valve_df))

        conn.close()

        if not temp_rows and not valve_rows:
            logger.info("No historical data found in database")
            return False

        final_checkpoint = checkpoint_manager.save_checkpoint("final_warmup")
        logger.info("Regular warmup completed! Final checkpoint: %s", final_checkpoint)

        return True

    except sqlite3.Error as err:
        logger.warning("Warning: Failed to load data from database: %s", err)
        return False


def gradual_warmup_from_database(
    db_path,
    batch_size=10,
    checkpoint_interval=1000,
    max_samples=None,
    n_clusters=3,
    outlier_percentile=95,
):
    """
    Gradually warmup models from database with checkpointing.
    Processes data in batches and saves checkpoints periodically.
    """
    logger.info("Starting gradual warmup from database: %s", db_path)
    logger.info("Batch size: %s, Checkpoint interval: %s", batch_size, checkpoint_interval)

    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        # Get total counts
        cursor.execute("SELECT COUNT(*) FROM temp_samples")
        total_temp = cursor.fetchone()[0]

        cursor.execute("SELECT COUNT(*) FROM valve_samples")
        total_valve = cursor.fetchone()[0]

        logger.info("Database contains: %s temperature samples, %s valve samples", total_temp, total_valve)

        # Process temperature samples in batches
        if total_temp > 0:
            limit = max_samples if max_samples else total_temp
            offset = 0

            while offset < limit:
                current_batch = min(batch_size, limit - offset)

                cursor.execute(
                    """
					SELECT id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
					FROM temp_samples
					ORDER BY timestamp ASC
					LIMIT ? OFFSET ?
				""",
                    (current_batch, offset),
                )

                temp_rows = cursor.fetchall()
                if temp_rows:
                    temp_samples = []
                    for row in temp_rows:
                        temp_samples.append(
                            {
                                "id": row[0],
                                "timestamp": row[1],
                                "sensor_type": row[2],
                                "sensor_number": row[3],
                                "fan_on": bool(row[4]),
                                "temperature": row[5],
                                "heater_power": row[6],
                                "anomalies": [],
                            }
                        )

                    temp_df = pd.DataFrame(temp_samples)
                    update_recent_history("temperature", temp_df)
                    refresh_temperature_baseline_model(
                        n_clusters=n_clusters,
                        outlier_percentile=outlier_percentile,
                    )

                    # Update training state
                    checkpoint_manager.update_training_state(len(temp_samples))

                    logger.info("Temperature batch: %s/%s samples", (offset + len(temp_samples)), min(limit, total_temp))

                    # Save checkpoint periodically
                    if (offset + len(temp_samples)) % checkpoint_interval == 0:
                        checkpoint_manager.save_checkpoint()

                offset += current_batch

                # Small delay to prevent overwhelming
                time.sleep(0.1)

        # Process valve samples in batches
        if total_valve > 0:
            limit = max_samples if max_samples else total_valve
            offset = 0

            while offset < limit:
                current_batch = min(batch_size, limit - offset)

                cursor.execute(
                    """
					SELECT id, timestamp, sensor_type, valve_number, is_open, flow_rate
					FROM valve_samples
					ORDER BY timestamp ASC
					LIMIT ? OFFSET ?
				""",
                    (current_batch, offset),
                )

                valve_rows = cursor.fetchall()
                if valve_rows:
                    valve_samples = []
                    for row in valve_rows:
                        valve_samples.append(
                            {
                                "id": row[0],
                                "timestamp": row[1],
                                "sensor_type": row[2],
                                "sensor_number": row[3],
                                "is_open": bool(row[4]),
                                "flow_rate": row[5],
                                "anomalies": [],
                            }
                        )

                    valve_df = pd.DataFrame(valve_samples)
                    update_recent_history("valve", valve_df)

                    # Update training state
                    checkpoint_manager.update_training_state(len(valve_samples))

                    logger.info("Valve batch: %s/%s samples", (offset + len(valve_samples)), (min(limit, total_valve)))

                    # Save checkpoint periodically
                    if (offset + len(valve_samples)) % checkpoint_interval == 0:
                        checkpoint_manager.save_checkpoint()

                offset += current_batch

                # Small delay to prevent overwhelming
                time.sleep(0.1)

        conn.close()

        refresh_temperature_baseline_model(
            n_clusters=n_clusters,
            outlier_percentile=outlier_percentile,
        )

        # Final checkpoint
        final_checkpoint = checkpoint_manager.save_checkpoint("final_warmup")
        logger.info("Gradual warmup completed! Final checkpoint: %s", final_checkpoint)

        return True

    except sqlite3.Error as err:
        logger.warning("Database warmup failed: %s", err)
        return False


def load_latest_checkpoint():
    """
    Load the most recent checkpoint if available.
    """
    latest_checkpoint = checkpoint_manager.get_latest_checkpoint()
    if latest_checkpoint:
        logger.info("Found latest checkpoint: %s", latest_checkpoint)
        return checkpoint_manager.load_checkpoint(latest_checkpoint)
    else:
        logger.info("No checkpoints found, starting fresh")
        return False

def initialize_checkpointing(args):
    if args.warmup_db:
        # Perform gradual warmup if requested
        if args.gradual_warmup:
            logger.info("Starting gradual warmup with checkpointing...")
            if not gradual_warmup_from_database(
                args.warmup_db,
                batch_size=args.warmup_batch_size,
                checkpoint_interval=args.checkpoint_interval,
                max_samples=args.max_warmup_samples,
                n_clusters=args.clusters,
                outlier_percentile=args.percentile,
            ):
                logger.error("Gradual warmup failed")
                return

        # Perform regular database warmup if specified (but not gradual)
        if not args.gradual_warmup:
            logger.info("Attempting to warmup from database: %s", args.warmup_db)
            if not warmup_from_database(
                args.warmup_db,
                args.warmup_db_limit,
                n_clusters=args.clusters,
                outlier_percentile=args.percentile,
            ):
                logger.error("Database warmup failed, continuing without warmup data")
