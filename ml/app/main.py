"""
Artifact: KMeans Anomaly Detection Service
Description: Runs KMeans anomaly detection on TempSample data and can serve results over HTTP for the ingest pipeline
Authors: Jacob Kice, Barrett Brown, Adam Berry
Date Created: 02/24/2026
Date Revised: 03/28/2026
Preconditions: Input data is valid json with temperature sample fields and sklearn dependencies are installed
Postconditions: Returns anomaly results either to stdout in CLI mode or as json in API mode
Possible errors: FileNotFoundError, JSONDecodeError, KeyError for missing fields, ValueError for bad request payloads
Side effects: Starts an HTTP server in API mode
Invariants: KMeans logic stays based on fan_on, temperature, and heater_power
Known faults: Small sample sets can reduce cluster count to avoid fit failures
"""

import argparse
import errno
import os
from http.server import ThreadingHTTPServer
import logging
from utilities.checkpointing import (
    checkpoint_manager,
    initialize_checkpointing,
    load_latest_checkpoint,
    TEMPERATURE_HISTORY_LIMIT,
)
from utilities.mlRequestHandler import MLRequestHandler

logger = logging.getLogger(__name__)
DEFAULT_SERVER_HOST = "127.0.0.1"
DEFAULT_PORT_NUM = 8000

def serve(host=DEFAULT_SERVER_HOST, port=DEFAULT_PORT_NUM, clusters=3, percentile=95):
    """
    Starts the HTTP API server.
    Args:
            host: Interface to bind.
            port: Port to listen on.
            clusters: KMeans cluster count.
            percentile: Outlier percentile threshold.
    Returns:
            None.
    """
    # Generate class at runtime instead of hardcoding
    # Note: type with 3 args is a constructor type(name, bases, dict)
    handler = type(
        "ConfiguredMLRequestHandler",
        (MLRequestHandler,),
        {"clusters": clusters, "percentile": percentile},
    )
    ThreadingHTTPServer.allow_reuse_address = True
    try:
        server = ThreadingHTTPServer((host, port), handler)
    except OSError as err:
        if err.errno == errno.EADDRINUSE:
            raise RuntimeError(
                f"ML API could not bind to http://{host}:{port} because the port is already in use. "
                f"Try stopping the existing process or run `make dev ML_PORT={port + 1}`."
            ) from err
        if err.errno == errno.EADDRNOTAVAIL:
            raise RuntimeError(
                f"ML API could not bind to http://{host}:{port} because that host address is not available on this machine."
            ) from err
        raise
    logger.info("ML API listening on http://%s:%s", host, port)
    server.serve_forever()


def get_cli_flags():
    """ Pulls in the runtime flags and pass them through to main """
    parser = argparse.ArgumentParser(
        description="Anomaly detection for typed ingest batches."
    )
    parser.add_argument(
        "input", nargs="?", help="Path to JSON file with batch data, or - for stdin."
    )
    parser.add_argument(
        "--clusters",
        type=int,
        default=3,
        help="Number of KMeans clusters for temperature.",
    )
    parser.add_argument(
        "--percentile",
        type=float,
        default=95,
        help="Percentile for temperature outlier threshold.",
    )
    parser.add_argument(
        "--serve",
        action="store_true",
        help="Start HTTP API mode instead of CLI file mode.",
    )
    parser.add_argument(
        "--host",
        default=os.environ.get("ML_HOST", "127.0.0.1"),
        help="Host interface for HTTP API mode.",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.environ.get("ML_PORT", "8000")),
        help="Port for HTTP API mode.",
    )
    parser.add_argument(
        "--warmup-db",
        help="Path to SQLite database file for extracting historical data for warmup.",
    )
    parser.add_argument(
        "--warmup-db-limit",
        type=int,
        default=TEMPERATURE_HISTORY_LIMIT,
        help=f"Maximum number of samples to extract from database per event type (default: {TEMPERATURE_HISTORY_LIMIT}).",
    )
    parser.add_argument(
        "--gradual-warmup",
        action="store_true",
        help="Enable gradual warmup with checkpointing.",
    )
    parser.add_argument(
        "--warmup-batch-size",
        type=int,
        default=10,
        help="Batch size for gradual warmup (default: 10).",
    )
    parser.add_argument(
        "--checkpoint-interval",
        type=int,
        default=1000,
        help="Save checkpoint every N samples (default: 1000).",
    )
    parser.add_argument(
        "--max-warmup-samples",
        type=int,
        help="Maximum samples to process during warmup (optional).",
    )
    parser.add_argument("--load-checkpoint", help="Load specific checkpoint file.")
    parser.add_argument(
        "--status", action="store_true", help="Show current training status and exit."
    )
    parser.add_argument(
        "--plot-output",
        help="Optional image path for a temperature cluster/anomaly visualization.",
    )
    parser.add_argument(
        "--plot-title", help="Optional title for the saved temperature visualization."
    )

    return parser.parse_args()

def main():
    """
    Main entry point: either run CLI analysis or start the HTTP API.
    Args:
            None, uses command line arguments.
    """
    args = get_cli_flags()

    # Always try to load the latest checkpoint on startup (unless specifically loading a different one)
    if not args.load_checkpoint:
        load_latest_checkpoint()

    # Handle status request
    if args.status:
        status = checkpoint_manager.get_status()
        print("Current Training Status:")
        print(f'   Epoch: {status["epoch"]}')
        print(f'   Samples Processed: {status["samples_processed"]}')
        print(f'   History Sizes: {status["history_sizes"]}')
        print(f'   Last Checkpoint: {status["last_checkpoint"]}')
        print(f'   Models Loaded: {status["models_loaded"]}')
        return

    # Load specific checkpoint if requested (overrides auto-loading)
    if args.load_checkpoint:
        if not checkpoint_manager.load_checkpoint(args.load_checkpoint):
            logger.error("Gradual warmup failed")

    # if we are asked to do warmup
    initialize_checkpointing(args)

    # Spins up the test server
    if args.serve:
        serve(
            host=args.host,
            port=args.port,
            clusters=args.clusters,
            percentile=args.percentile,
        )
        return

    if not args.input:
        logger.error("input is required unless --serve is used")

if __name__ == "__main__":
    main()
