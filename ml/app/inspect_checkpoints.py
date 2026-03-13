#!/usr/bin/env python3
"""
Checkpoint Inspector
Shows the contents of checkpoint files.
"""

import argparse
import os
import pickle
from pprint import pformat


APP_DIR = os.path.dirname(os.path.abspath(__file__))
CHECKPOINT_DIR = os.path.normpath(os.path.join(APP_DIR, '..', 'checkpoints'))


def summarize_model(name, model_state):
    """Build a readable summary for a stored model payload."""
    if not model_state:
        return f"{name}: None"

    if isinstance(model_state, dict):
        lines = [f"{name}: loaded"]
        if 'baseline_samples' in model_state:
            lines.append(f"  baseline_samples: {model_state['baseline_samples']}")
        if 'n_clusters' in model_state:
            lines.append(f"  n_clusters: {model_state['n_clusters']}")
        if 'threshold' in model_state:
            lines.append(f"  threshold: {model_state['threshold']}")
        if 'outlier_percentile' in model_state:
            lines.append(f"  outlier_percentile: {model_state['outlier_percentile']}")
        if 'fitted_at' in model_state:
            lines.append(f"  fitted_at: {model_state['fitted_at']}")
        lines.append(f"  keys: {list(model_state.keys())}")
        return "\n".join(lines)

    return f"{name}: loaded ({type(model_state).__name__})"


def inspect_checkpoint(checkpoint_path):
    """Inspect a single checkpoint file."""
    print(f"\nInspecting: {os.path.basename(checkpoint_path)}")
    print("-" * 60)

    with open(checkpoint_path, 'rb') as handle:
        data = pickle.load(handle)

    print(f"Path: {checkpoint_path}")
    print(f"Top-level keys: {list(data.keys())}")
    print(f"Saved at: {data.get('saved_at', 'unknown')}")

    state = data.get('training_state', {})
    print("\ntraining_state:")
    print(pformat(state))

    models = data.get('models', {})
    print("\nmodels:")
    for model_name, model_state in models.items():
        print(summarize_model(model_name, model_state))

    history = data.get('recent_history', {})
    history_sizes = {name: len(frame) for name, frame in history.items()}
    print("\nrecent_history sizes:")
    print(pformat(history_sizes))


def main():
    parser = argparse.ArgumentParser(description='Inspect saved warmup checkpoint files.')
    parser.add_argument('checkpoint', nargs='?', help='Optional path to a single checkpoint file.')
    args = parser.parse_args()

    if args.checkpoint:
        inspect_checkpoint(os.path.abspath(args.checkpoint))
        return

    if not os.path.exists(CHECKPOINT_DIR):
        print("No checkpoints directory found")
        return

    checkpoints = [f for f in os.listdir(CHECKPOINT_DIR) if f.endswith('.pkl')]
    checkpoints.sort()

    print(f"Found {len(checkpoints)} checkpoint files in {CHECKPOINT_DIR}:")
    for checkpoint_name in checkpoints:
        print(f"  - {checkpoint_name}")

    for checkpoint_name in checkpoints:
        checkpoint_path = os.path.join(CHECKPOINT_DIR, checkpoint_name)
        inspect_checkpoint(checkpoint_path)


if __name__ == '__main__':
    main()
