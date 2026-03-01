'''
Artifact: KMeans Anomaly Detection Service
Description: Runs KMeans anomaly detection on TempSample data and can serve results over HTTP for the ingest pipeline
Author: Jacob Kice, Barrett Brown
Date Created: 02/24/2026
Date Revised: 03/01/2026
Preconditions: Input data is valid json with temperature sample fields and sklearn dependencies are installed
Postconditions: Returns anomaly results either to stdout in CLI mode or as json in API mode
Possible errors: FileNotFoundError, JSONDecodeError, KeyError for missing fields, ValueError for bad request payloads
Side effects: Starts an HTTP server in API mode
Invariants: KMeans logic stays based on fan_on, temperature, and heater_power
Known faults: Small sample sets can reduce cluster count to avoid fit failures
'''

import argparse
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import numpy as np
import pandas as pd
from sklearn.cluster import KMeans


def load_samples_from_json(json_source):
	'''
	Loads TempSample data from a JSON file or stdin.
	Args:
		json_source: Path to JSON file or '-' for stdin.
	Returns:
		DataFrame of TempSample data.
	'''
	if json_source == '-':
		# Read from stdin if '-' is provided
		data = json.load(sys.stdin)
	else:
		# Read from file
		with open(json_source, 'r') as f:
			data = json.load(f)
	return dataframe_from_samples(data)


def dataframe_from_samples(data):
	'''
	Converts either the raw sample list or the ingest request body into a DataFrame.
	Args:
		data: Either a list of sample dicts or an object with a samples field.
	Returns:
		DataFrame built from the provided samples.
	'''
	if isinstance(data, dict) and 'samples' in data:
		data = data['samples']
	if not isinstance(data, list):
		raise ValueError('Request body must be a list of samples or an object with a samples field')
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
	if df.empty:
		return df.copy()

	# Trim to relevant features
	trimmed = df.loc[:, ['fan_on', 'temperature', 'heater_power']].copy()
	
	# Convert boolean to int for clustering
	if 'fan_on' in trimmed:
		trimmed['fan_on'] = trimmed['fan_on'].astype(int)

	# Initialize Kmeans Object
	cluster_count = min(max(1, len(trimmed)), n_clusters)
	kmeans = KMeans(n_clusters=cluster_count, random_state=42)
	kmeans.fit(trimmed)
	
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


def build_anomaly_response(result_df):
	'''
	Builds a stable json response for the ingest service.
	Args:
		result_df: DataFrame produced by kmeans_anomaly_detection
	Returns:
		Dictionary with top level anomaly summary and per sample details.
	'''
	if result_df.empty:
		return {
			'model': 'kmeans',
			'has_anomaly': False,
			'label': '',
			'labels': [],
			'score': None,
			'sample_count': 0,
			'anomaly_count': 0,
			'anomalies': [],
		}

	anomalies = []
	for index, row in result_df.iterrows():
		is_anomaly = bool(row.get('kmeans_outlier', 0))
		score = float(row.get('kmeans_distance', 0.0))
		entry = {
			'index': int(index),
			'id': int(row['id']) if 'id' in row and pd.notna(row['id']) else None,
			'timestamp': row.get('timestamp'),
			'is_anomaly': is_anomaly,
			'label': 'kmeans_outlier' if is_anomaly else '',
			'score': score,
		}
		if is_anomaly:
			anomalies.append(entry)

	max_score = float(result_df['kmeans_distance'].max()) if 'kmeans_distance' in result_df else None
	has_anomaly = len(anomalies) > 0
	return {
		'model': 'kmeans',
		'has_anomaly': has_anomaly,
		'label': 'kmeans_outlier' if has_anomaly else '',
		'labels': ['kmeans_outlier'] if has_anomaly else [],
		'score': max_score,
		'sample_count': int(len(result_df)),
		'anomaly_count': int(len(anomalies)),
		'anomalies': anomalies,
	}


def analyze_samples(data, n_clusters=3, outlier_percentile=95):
	'''
	Runs the end to end analysis path for either CLI or API usage.
	Args:
		data: Raw sample list or request body object.
		n_clusters: Number of clusters for KMeans.
		outlier_percentile: Outlier percentile threshold.
	Returns:
		Stable anomaly response dictionary.
	'''
	df = dataframe_from_samples(data)
	result = kmeans_anomaly_detection(df, n_clusters=n_clusters, outlier_percentile=outlier_percentile)
	return build_anomaly_response(result)


class MLRequestHandler(BaseHTTPRequestHandler):
	'''
	Simple HTTP handler that exposes the anomaly detection over POST.
	'''

	clusters = 3
	percentile = 95

	# Handles incoming POST requests
	def do_POST(self):
		# Accept only / and /analyze routes
		if self.path not in ('/', '/analyze'):
			self.write_json({'error': 'not_found'}, 404)
			return

		# Read request body
		content_length = int(self.headers.get('Content-Length', '0'))
		body = self.rfile.read(content_length) if content_length > 0 else b''
		try:
			# Analyze payload
			payload = json.loads(body or b'{}') # Parse the body or if it is empty fallback to an empty one so no issues with next line
			response = analyze_samples(payload, n_clusters=self.clusters, outlier_percentile=self.percentile)
		except (json.JSONDecodeError, ValueError, KeyError) as err:
			self.write_json({'error': str(err)}, 400)
			return
		except Exception as err:
			self.write_json({'error': str(err)}, 500)
			return

		# Return JSON back to client
		self.write_json(response, 200)

	# Handles incoming GET requests (Health checks mainly for down the line)
	def do_GET(self):
		if self.path not in ('/', '/health'):
			self.write_json({'error': 'not_found'}, 404)
			return
		self.write_json({'status': 'ok', 'model': 'kmeans'}, 200)

	# Just overwritten the default server access logs so the console isn't spammed
	def log_message(self, format, *args):
		return

	# Helper function that writes a JSON response with status code / headers
	def write_json(self, payload, status_code):
		body = json.dumps(payload).encode('utf-8')
		self.send_response(status_code)
		self.send_header('Content-Type', 'application/json')
		self.send_header('Content-Length', str(len(body)))
		self.end_headers()
		self.wfile.write(body)


def serve(host='127.0.0.1', port=8000, clusters=3, percentile=95):
	'''
	Starts the HTTP API server.
	Args:
		host: Interface to bind.
		port: Port to listen on.
		clusters: KMeans cluster count.
		percentile: Outlier percentile threshold.
	Returns:
		None.
	'''
	# Generate class at runtime instead of hardcoding
 	# Note: type with 3 bits is a constructor type(name, bases, dict)
	handler = type(
		'ConfiguredMLRequestHandler',
		(MLRequestHandler,),
		{'clusters': clusters, 'percentile': percentile},
	)
	server = ThreadingHTTPServer((host, port), handler)
	print(f'ML API listening on http://{host}:{port}')
	server.serve_forever()


def main():
	'''
	Main entry point: either run CLI analysis or start the HTTP API.
	Args:
		None, uses command line arguments.
	Returns:
		None.
	'''
	parser = argparse.ArgumentParser(description='KMeans anomaly detection for TempSample data.')
	parser.add_argument('input', nargs='?', help='Path to JSON file with TempSample data, or - for stdin.')
	parser.add_argument('--clusters', type=int, default=3, help='Number of KMeans clusters.')
	parser.add_argument('--percentile', type=float, default=95, help='Percentile for outlier threshold.')
	parser.add_argument('--serve', action='store_true', help='Start HTTP API mode instead of CLI file mode.')
	parser.add_argument('--host', default='127.0.0.1', help='Host for API mode.')
	parser.add_argument('--port', type=int, default=8000, help='Port for API mode.')
	args = parser.parse_args()

	# Serve used as a "Hey this is in live production" skip over the manual just this testing
	if args.serve:
		serve(host=args.host, port=args.port, clusters=args.clusters, percentile=args.percentile)
		return

	if not args.input:
		parser.error('input is required unless --serve is used')

	df = load_samples_from_json(args.input)
	result = kmeans_anomaly_detection(df, n_clusters=args.clusters, outlier_percentile=args.percentile)
	print(result)


if __name__ == '__main__':
	main()
