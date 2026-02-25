'''
Artifact: KMeans Anomaly Detection
Description: Runs KMeans anomaly detection on TempSample data, adapted for fanout pipeline input.
Author: Jacob Kice
Date Created: 02/24/2026
Date Revised: Not Applicable
Preconditions: Input file is located in pre-defined location, of type json, with appropriate internal format (list of TempSample dicts)
Postconditions: Prints DataFrame with anomaly detection results
Possible errors: FileNotFoundError, JSONDecodeError, KeyError for missing fields
Side effects: None (prints results to stdout)
Invariants: Not Applicable
Known faults: None
'''


# Import modules
import pandas as pd
from sklearn.cluster import KMeans
import numpy as np
import sys
import json
import argparse


def load_samples_from_json(json_source):
	'''
	Loads TempSample data from a JSON file or stdin.
	Args:
		json_source: Path to JSON file or '-' for stdin.
	Returns:
		DataFrame of TempSample data (list of dicts with keys: fan_on, temperature, heater_power, etc.)
	'''
	if json_source == '-':
		# Read from stdin if '-' is provided
		data = json.load(sys.stdin)
	else:
		# Read from file
		with open(json_source, 'r') as f:
			data = json.load(f)
	return pd.DataFrame(data)


def kmeans_anomaly_detection(df, n_clusters=3, outlier_percentile=95):
	'''
	Runs KMeans clustering and flags outliers based on distance to nearest cluster center.
	Args:
		df: DataFrame of TempSample data
		n_clusters: Number of clusters for KMeans
		outlier_percentile: Percentile threshold for outlier detection
	Returns:
		DataFrame with kmeans_outlier and kmeans_distance columns
	'''
	# Trim to relevant features
	trimmed = df.loc[:, ['fan_on', 'temperature', 'heater_power']]
	# Convert boolean to int for clustering
	if 'fan_on' in trimmed:
		trimmed['fan_on'] = trimmed['fan_on'].astype(int)
	# Initialize KMeans object
	kmeans = KMeans(n_clusters=n_clusters, random_state=42)
	kmeans.fit(trimmed)  # Fit KMeans to trimmed data
	# Get distances to nearest cluster center
	distances = kmeans.transform(trimmed).min(axis=1)
	# Set threshold for outlier distance
	threshold = np.percentile(distances, outlier_percentile)
	# Mark as outlier if distance is at or above threshold
	kmeans_outlier = (distances >= threshold).astype(int)
	df = df.copy()
	df['kmeans_outlier'] = kmeans_outlier
	df['kmeans_distance'] = distances
	return df


def main():
	'''
	Main entry point: parses arguments, loads data, runs anomaly detection, prints results.
	Args:
		None (uses command-line arguments)
	Returns:
		None (prints results to stdout)
	'''
	parser = argparse.ArgumentParser(description="KMeans anomaly detection for TempSample data.")
	parser.add_argument('input', help="Path to JSON file with TempSample data, or '-' for stdin.")
	parser.add_argument('--clusters', type=int, default=3, help="Number of KMeans clusters.")
	parser.add_argument('--percentile', type=float, default=95, help="Percentile for outlier threshold.")
	args = parser.parse_args()

	# Load TempSample data
	df = load_samples_from_json(args.input)
	# Run KMeans anomaly detection
	result = kmeans_anomaly_detection(df, n_clusters=args.clusters, outlier_percentile=args.percentile)
	# Print results
	print(result)


# Run main if executed as a script
if __name__ == '__main__':
	main()
