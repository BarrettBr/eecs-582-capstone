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

HISTORY_LIMIT = 64
RECENT_HISTORY = {
	'temperature': pd.DataFrame(),
	'valve': pd.DataFrame(),
}


def load_samples_from_json(json_source):
	'''
	Loads TempSample data from a JSON file or stdin.
	Args:
		json_source: Path to JSON file or '-' for stdin.
	Returns:
		Tuple of event_type and DataFrame of sample data.
	'''
	if json_source == '-':
		# Read from stdin if '-' is provided
		data = json.load(sys.stdin)
	else:
		# Read from file
		with open(json_source, 'r', encoding='utf-8') as f:
			data = json.load(f)
	return dataframe_from_request(data)


def dataframe_from_request(data):
	'''
	Converts either the raw sample list or the ingest request body into a DataFrame.
	Args:
		data: Either a list of sample dicts or an object with event_type and samples fields.
	Returns:
		Tuple of event_type and DataFrame built from the provided samples.
	'''
	if isinstance(data, list):
		return 'temperature', pd.DataFrame(data)
	if not isinstance(data, dict):
		raise ValueError('Request body must be an object with event_type and samples')
	event_type = data.get('event_type')
	if not isinstance(event_type, str) or not event_type.strip():
		raise ValueError('Request body must include event_type')
	samples = data.get('samples')
	if not isinstance(samples, list):
		raise ValueError('Request body must include samples as a list')
	return event_type, pd.DataFrame(samples)


def kmeans_anomaly_detection(df, n_clusters=3, outlier_percentile=95):
	'''
	Runs KMeans clustering and flags outliers based on distance to nearest cluster center.
	Args:
		df: DataFrame of TempSample data
		n_clusters: Number of clusters for KMeans
		outlier_percentile: Percentile threshold for outlier detection
	Returns:
		DataFrame with detected_anomaly and anomaly_score columns
	'''
	if df.empty:
		return df.copy()

	# Trim to relevant features
	trimmed = df.loc[:, ['fan_on', 'temperature', 'heater_power']].copy()

	# Convert boolean to int for clustering
	trimmed['fan_on'] = trimmed['fan_on'].astype(int)

	# Very small or low-variety batches are not meaningful for clustering and
	# tend to mark nearly every batch as anomalous. Treat them as "no anomaly".
	if len(trimmed) < 5:
		df = df.copy()
		df['detected_anomaly'] = 0
		df['anomaly_score'] = 0.0
		df['anomaly_label'] = ''
		return df

	unique_count = len(trimmed.drop_duplicates())
	if unique_count < 3:
		df = df.copy()
		df['detected_anomaly'] = 0
		df['anomaly_score'] = 0.0
		df['anomaly_label'] = ''
		return df

	# Initialize Kmeans Object
	cluster_count = min(max(1, len(trimmed)), n_clusters, unique_count)
	kmeans = KMeans(n_clusters=cluster_count, random_state=42)
	kmeans.fit(trimmed)

	# Get distances to nearest cluster center
	distances = kmeans.transform(trimmed).min(axis=1)

	# Set threshold for outlier distance
	threshold = np.percentile(distances, outlier_percentile)

	# Mark as outlier if distance is at or above threshold
	df = df.copy()
	df['detected_anomaly'] = (distances > threshold).astype(int)
	df['anomaly_score'] = distances
	df['anomaly_label'] = np.where(df['detected_anomaly'] == 1, 'kmeans_outlier', '')
	return df


def valve_anomaly_detection(df):
	'''
	Runs a simple rule-based anomaly detector for ValveSample data.
	Args:
		df: DataFrame of ValveSample data
	Returns:
		DataFrame with detected_anomaly and anomaly_score columns
	'''
	if df.empty:
		return df.copy()

	flow_rate = pd.to_numeric(df.get('flow_rate', 0.0), errors='coerce').fillna(0.0)
	is_open = df.get('is_open', False)
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
			labels.append('valve_flow_while_closed')
		elif open_flow:
			labels.append('valve_flow_out_of_range')
		else:
			labels.append('')

	df = df.copy()
	df['detected_anomaly'] = flags.astype(int)
	df['anomaly_score'] = flow_rate.abs()
	df['anomaly_label'] = labels
	return df


def build_anomaly_response(result_df, event_type, model):
	'''
	Builds a stable json response for the ingest service.
	Args:
		result_df: DataFrame produced by one of the anomaly detection helpers
		event_type: Event type associated with the analyzed batch
		model: Model identifier for the response
	Returns:
		Dictionary with top level anomaly summary and per sample details.
	'''
	if result_df.empty:
		return {
			'model': model,
			'event_type': event_type,
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
		is_anomaly = bool(row.get('detected_anomaly', 0))
		label = row.get('anomaly_label', '') if is_anomaly else ''
		score = float(row.get('anomaly_score', 0.0))
		entry = {
			'index': int(index),
			'id': int(row['id']) if 'id' in row and pd.notna(row['id']) else None,
			'timestamp': row.get('timestamp'),
			'is_anomaly': is_anomaly,
			'label': label,
			'score': score,
		}
		if is_anomaly:
			anomalies.append(entry)

	labels = sorted({entry['label'] for entry in anomalies if entry['label']})
	max_score = float(result_df['anomaly_score'].max()) if 'anomaly_score' in result_df else None
	has_anomaly = len(anomalies) > 0
	return {
		'model': model,
		'event_type': event_type,
		'has_anomaly': has_anomaly,
		'label': labels[0] if labels else '',
		'labels': labels,
		'score': max_score,
		'sample_count': int(len(result_df)),
		'anomaly_count': int(len(anomalies)),
		'anomalies': anomalies,
	}


def update_recent_history(event_type, df):
	'''
	Stores a rolling history window so unsupervised detection can compare a small
	current batch against recent baseline behavior.
	Args:
		event_type: Event family for the provided samples.
		df: Current batch DataFrame.
	Returns:
		None.
	'''
	if event_type not in RECENT_HISTORY:
		RECENT_HISTORY[event_type] = pd.DataFrame()
	combined = pd.concat([RECENT_HISTORY[event_type], df.copy()], ignore_index=True)
	RECENT_HISTORY[event_type] = combined.tail(HISTORY_LIMIT).reset_index(drop=True)


def build_temperature_analysis_frame(df):
	'''
	Combines recent temperature history with the current batch so KMeans has
	enough baseline context while only the current rows are reported back.
	Args:
		df: Current temperature batch.
	Returns:
		DataFrame containing history plus current rows, with a marker column.
	'''
	history = RECENT_HISTORY.get('temperature', pd.DataFrame()).copy()
	if history.empty:
		current = df.copy()
		current['__is_current'] = True
		return current

	history['__is_current'] = False
	current = df.copy()
	current['__is_current'] = True
	return pd.concat([history, current], ignore_index=True)


def analyze_temperature(df, n_clusters=3, outlier_percentile=95):
	'''
	Runs the end to end temperature analysis path.
	Args:
		df: DataFrame of temperature samples.
		n_clusters: Number of clusters for KMeans.
		outlier_percentile: Outlier percentile threshold.
	Returns:
		Stable anomaly response dictionary.
	'''
	required = {'fan_on', 'temperature', 'heater_power'}
	missing = sorted(required.difference(df.columns))
	if missing:
		raise ValueError(f'temperature samples missing fields: {", ".join(missing)}')

	analysis_df = build_temperature_analysis_frame(df)
	result = kmeans_anomaly_detection(analysis_df, n_clusters=n_clusters, outlier_percentile=outlier_percentile)
	current_result = result[result['__is_current']].drop(columns=['__is_current'], errors='ignore')
	update_recent_history('temperature', df)
	return build_anomaly_response(current_result, 'temperature', 'kmeans')


def analyze_valve(df):
	'''
	Runs the end to end valve analysis path.
	Args:
		df: DataFrame of valve samples.
	Returns:
		Stable anomaly response dictionary.
	'''
	required = {'is_open', 'flow_rate'}
	missing = sorted(required.difference(df.columns))
	if missing:
		raise ValueError(f'valve samples missing fields: {", ".join(missing)}')
	result = valve_anomaly_detection(df)
	return build_anomaly_response(result, 'valve', 'rule_engine')


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
	event_type, df = dataframe_from_request(data)
	if event_type == 'temperature':
		return analyze_temperature(df, n_clusters=n_clusters, outlier_percentile=outlier_percentile)
	if event_type == 'valve':
		return analyze_valve(df)
	raise ValueError(f'unsupported event_type: {event_type}')


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
			payload = json.loads(body or b'{}')
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
		self.write_json({'status': 'ok', 'model': 'multi_type'}, 200)

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
	# Note: type with 3 args is a constructor type(name, bases, dict)
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
	parser = argparse.ArgumentParser(description='Anomaly detection for typed ingest batches.')
	parser.add_argument('input', nargs='?', help='Path to JSON file with batch data, or - for stdin.')
	parser.add_argument('--clusters', type=int, default=3, help='Number of KMeans clusters for temperature.')
	parser.add_argument('--percentile', type=float, default=95, help='Percentile for temperature outlier threshold.')
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

	# CLI mode keeps working for local batch testing
	event_type, df = load_samples_from_json(args.input)
	if event_type == 'temperature':
		result = analyze_temperature(df, n_clusters=args.clusters, outlier_percentile=args.percentile)
	elif event_type == 'valve':
		result = analyze_valve(df)
	else:
		raise ValueError(f'unsupported event_type: {event_type}')
	print(json.dumps(result))


if __name__ == '__main__':
	main()
