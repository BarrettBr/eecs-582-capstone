
'''
Artifact: Clustering Methods Test Suite
Description: Implements and evaluates four common clustering algorithms (KMeans, Hierarchical, DBSCAN, GMM)
             on TempSample data from test_samples.json.
             Reports best parameters and scores, and visualizes results.
Author: Jacob Kice
Date Created: 03/11/2026
Preconditions: test_samples.json exists and sklearn, matplotlib dependencies are installed
Postconditions: Prints best parameters, scores, and cluster visualizations for each method
Possible errors: FileNotFoundError, JSONDecodeError, ValueError for missing fields, sklearn exceptions for invalid parameter ranges
Side effects: Opens matplotlib windows for cluster plots
Invariants: Feature selection matches main.py (fan_on, temperature, heater_power)
Known faults: Small sample sets may limit cluster count or metric reliability
'''


# Standard library imports
import json

# Third-party imports
import numpy as np
import pandas as pd
from sklearn.cluster import KMeans, AgglomerativeClustering, DBSCAN
from sklearn.mixture import GaussianMixture
from sklearn.metrics import silhouette_score, davies_bouldin_score, calinski_harabasz_score
from sklearn.decomposition import PCA
import matplotlib.pyplot as plt


def load_samples(path):
    '''
    Loads TempSample data from a JSON file.
    Args:
        path: Path to JSON file.
    Returns:
        DataFrame of sample data.
    '''
    with open(path, 'r', encoding='utf-8') as f:
        data = json.load(f)
    return pd.DataFrame(data)


def prepare_features(df):
    '''
    Extracts relevant features for clustering from sample DataFrame.
    Args:
        df: DataFrame of sample data.
    Returns:
        DataFrame of features (fan_on, temperature, heater_power).
    '''
    features = df.loc[:, ['fan_on', 'temperature', 'heater_power']].copy()
    features['fan_on'] = features['fan_on'].astype(int)
    return features

# KMeans clustering

def search_kmeans(features, cluster_range=(2, 6)):
    '''
    Runs KMeans clustering for a range of cluster counts and selects the best by Silhouette Score.
    Args:
        features: DataFrame of features.
        cluster_range: Tuple (min, max) clusters to test.
    Returns:
        Best cluster labels found.
    '''
    best_score = -1
    best_params = None
    best_labels = None
    for n_clusters in range(cluster_range[0], cluster_range[1]+1):
        model = KMeans(n_clusters=n_clusters, random_state=42)
        labels = model.fit_predict(features)
        # Ignore noise label (-1) for compatibility
        mask = labels != -1 if -1 in labels else np.ones_like(labels, dtype=bool)
        valid_features = features[mask]
        valid_labels = labels[mask]
        n_clusters_valid = len(set(valid_labels))
        if n_clusters_valid < 2 or len(valid_features) < 2:
            continue
        try:
            sil = silhouette_score(valid_features, valid_labels)
            if sil > best_score:
                best_score = sil
                best_params = {'n_clusters': n_clusters}
                best_labels = labels
        except Exception:
            continue
    print(f'KMeans best params: {best_params}, Silhouette Score: {best_score:.3f}')
    evaluate_clustering(features, best_labels, 'KMeans')
    visualize_clusters(features, best_labels, 'KMeans')
    return best_labels

# Hierarchical clustering (Agglomerative)

def search_hierarchical(features, cluster_range=(2, 6)):
    '''
    Runs Agglomerative Hierarchical clustering for a range of cluster counts and selects the best by Silhouette Score.
    Args:
        features: DataFrame of features.
        cluster_range: Tuple (min, max) clusters to test.
    Returns:
        Best cluster labels found.
    '''
    best_score = -1
    best_params = None
    best_labels = None
    for n_clusters in range(cluster_range[0], cluster_range[1]+1):
        model = AgglomerativeClustering(n_clusters=n_clusters)
        labels = model.fit_predict(features)
        mask = labels != -1 if -1 in labels else np.ones_like(labels, dtype=bool)
        valid_features = features[mask]
        valid_labels = labels[mask]
        n_clusters_valid = len(set(valid_labels))
        if n_clusters_valid < 2 or len(valid_features) < 2:
            continue
        try:
            sil = silhouette_score(valid_features, valid_labels)
            if sil > best_score:
                best_score = sil
                best_params = {'n_clusters': n_clusters}
                best_labels = labels
        except Exception:
            continue
    print(f'Hierarchical best params: {best_params}, Silhouette Score: {best_score:.3f}')
    evaluate_clustering(features, best_labels, 'Hierarchical')
    visualize_clusters(features, best_labels, 'Hierarchical')
    return best_labels

# DBSCAN clustering

def search_dbscan(features, eps_range=(0.5, 3.0), min_samples_range=(2, 4)):
    '''
    Runs DBSCAN clustering for a range of eps and min_samples values, selects best by Silhouette Score.
    Args:
        features: DataFrame of features.
        eps_range: Tuple (min, max) for eps parameter.
        min_samples_range: Tuple (min, max) for min_samples parameter.
    Returns:
        Best cluster labels found.
    '''
    best_score = -1
    best_params = None
    best_labels = None
    for eps in np.arange(eps_range[0], eps_range[1]+0.1, 0.5):
        for min_samples in range(min_samples_range[0], min_samples_range[1]+1):
            model = DBSCAN(eps=eps, min_samples=min_samples)
            labels = model.fit_predict(features)
            mask = labels != -1 if -1 in labels else np.ones_like(labels, dtype=bool)
            valid_features = features[mask]
            valid_labels = labels[mask]
            n_clusters_valid = len(set(valid_labels)) - (1 if -1 in labels else 0)
            if n_clusters_valid < 2 or len(valid_features) < 2:
                continue
            try:
                sil = silhouette_score(valid_features, valid_labels)
                if sil > best_score:
                    best_score = sil
                    best_params = {'eps': eps, 'min_samples': min_samples}
                    best_labels = labels
            except Exception:
                continue
    print(f'DBSCAN best params: {best_params}, Silhouette Score: {best_score:.3f}')
    evaluate_clustering(features, best_labels, 'DBSCAN')
    visualize_clusters(features, best_labels, 'DBSCAN')
    return best_labels

# Gaussian Mixture Model clustering

def search_gmm(features, cluster_range=(2, 6)):
    '''
    Runs Gaussian Mixture Model clustering for a range of cluster counts and selects best by Silhouette Score.
    Args:
        features: DataFrame of features.
        cluster_range: Tuple (min, max) clusters to test.
    Returns:
        Best cluster labels found.
    '''
    best_score = -1
    best_params = None
    best_labels = None
    for n_clusters in range(cluster_range[0], cluster_range[1]+1):
        model = GaussianMixture(n_components=n_clusters, random_state=42)
        labels = model.fit_predict(features)
        mask = labels != -1 if -1 in labels else np.ones_like(labels, dtype=bool)
        valid_features = features[mask]
        valid_labels = labels[mask]
        n_clusters_valid = len(set(valid_labels))
        if n_clusters_valid < 2 or len(valid_features) < 2:
            continue
        try:
            sil = silhouette_score(valid_features, valid_labels)
            if sil > best_score:
                best_score = sil
                best_params = {'n_clusters': n_clusters}
                best_labels = labels
        except Exception:
            continue
    print(f'GMM best params: {best_params}, Silhouette Score: {best_score:.3f}')
    evaluate_clustering(features, best_labels, 'GMM')
    visualize_clusters(features, best_labels, 'GMM')
    return best_labels

# Evaluation metrics

def evaluate_clustering(features, labels, method):
    '''
    Computes and prints clustering evaluation metrics for the given labels.
    Args:
        features: DataFrame of features.
        labels: Cluster assignments.
        method: String name of clustering method.
    Returns:
        None.
    '''
    # Ignore noise label (-1) for DBSCAN
    mask = labels != -1 if -1 in labels else np.ones_like(labels, dtype=bool)
    valid_features = features[mask]
    valid_labels = labels[mask]
    n_clusters = len(set(valid_labels)) - (1 if -1 in labels else 0)
    if n_clusters < 2 or len(valid_features) < 2:
        print(f'{method} evaluation: Not enough clusters or samples for metrics.')
        return
    try:
        sil = silhouette_score(valid_features, valid_labels)
        db = davies_bouldin_score(valid_features, valid_labels)
        ch = calinski_harabasz_score(valid_features, valid_labels)
        print(f'{method} Silhouette Score: {sil:.3f}')
        print(f'{method} Davies-Bouldin Index: {db:.3f}')
        print(f'{method} Calinski-Harabasz Index: {ch:.3f}')
    except Exception as e:
        print(f'{method} evaluation error: {e}')

# Visualization

def visualize_clusters(features, labels, method):
    '''
    Projects features to 2D using PCA and plots clusters.
    Args:
        features: DataFrame of features.
        labels: Cluster assignments.
        method: String name of clustering method.
    Returns:
        None.
    '''
    pca = PCA(n_components=2)
    reduced = pca.fit_transform(features)
    plt.figure(figsize=(6, 4))
    scatter = plt.scatter(reduced[:, 0], reduced[:, 1], c=labels, cmap='tab10', s=50, edgecolor='k')
    plt.title(f'{method} Clustering (PCA projection)')
    plt.xlabel('PCA 1')
    plt.ylabel('PCA 2')
    plt.colorbar(scatter, label='Cluster Label')
    plt.tight_layout()
    plt.show()

# Main test runner

def main():
    '''
    Main entry point: loads data, runs parameter search for each clustering method, prints and visualizes results.
    Args:
        None.
    Returns:
        None.
    '''
    df = load_samples('ml\\app\\test_samples.json')
    features = prepare_features(df)
    print('Features used for clustering:')
    print(features)
    print('\n--- KMeans ---')
    search_kmeans(features)
    print('\n--- Hierarchical ---')
    search_hierarchical(features)
    print('\n--- DBSCAN ---')
    search_dbscan(features)
    print('\n--- GMM ---')
    search_gmm(features)


if __name__ == '__main__':
    main()
