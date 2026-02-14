import pandas as pd
from sklearn.ensemble import IsolationForest
from sklearn.cluster import DBSCAN
from sklearn.neighbors import LocalOutlierFactor
from sklearn.cluster import KMeans
import matplotlib.pyplot as plt

data = pd.read_json("../MOCK_DATA_10K.json")

trimmed = data.loc[:, ['fan_on', 'temperature', 'heater_power']]

iso_forest = IsolationForest()
iso_forest.fit(trimmed)


# Get anomaly scores
scores = iso_forest.score_samples(trimmed)

# Predict outliers (-1: outlier, 1: inlier)
predictions = iso_forest.predict(trimmed)


# Add scores and predictions to DataFrame

# DBSCAN anomaly detection
dbscan = DBSCAN(eps=0.5, min_samples=5)
dbscan_labels = dbscan.fit_predict(trimmed)
# In DBSCAN, label -1 means outlier, others are cluster labels
dbscan_outlier = (dbscan_labels == -1).astype(int)  # 1 for outlier, 0 for inlier

# Local Outlier Factor (LOF)
lof = LocalOutlierFactor(n_neighbors=20, contamination='auto')
lof_labels = lof.fit_predict(trimmed)
# In LOF, label -1 means outlier, 1 means inlier
lof_outlier = (lof_labels == -1).astype(int)  # 1 for outlier, 0 for inlier

# K-Means anomaly detection
# We'll use distance to nearest cluster center as anomaly score
kmeans = KMeans(n_clusters=3, random_state=42)
kmeans.fit(trimmed)
distances = kmeans.transform(trimmed).min(axis=1)
# Mark as outlier if distance is in the top 5% (tunable)
import numpy as np
threshold = np.percentile(distances, 95)
kmeans_outlier = (distances > threshold).astype(int)  # 1 for outlier, 0 for inlier

# Add scores and predictions to DataFrame
scored_data = data.assign(
	anomaly_scores=scores,
	outlier=predictions,
	dbscan_outlier=dbscan_outlier,
	lof_outlier=lof_outlier,
	kmeans_outlier=kmeans_outlier
)





print(scored_data)
print("\nIsolation Forest outlier: -1 (anomaly), 1 (normal)")
print("DBSCAN outlier: 1 (anomaly), 0 (normal)")
print("LOF outlier: 1 (anomaly), 0 (normal)")
print("KMeans outlier: 1 (anomaly), 0 (normal) (top 5% farthest from cluster center)")



# Plot anomaly scores, color by outlier status for all four methods in a 2x2 grid
fig, axes = plt.subplots(2, 2, figsize=(14, 10))

# Isolation Forest
colors_if = scored_data["outlier"].map({1: "blue", -1: "red"})
axes[0, 0].scatter(scored_data["id"], scored_data["anomaly_scores"], c=colors_if, s=10)
axes[0, 0].set_xlabel("id")
axes[0, 0].set_ylabel("anomaly_scores")
axes[0, 0].set_title("Isolation Forest (Red = Outlier)")

# DBSCAN
colors_db = scored_data["dbscan_outlier"].map({0: "blue", 1: "red"})
axes[0, 1].scatter(scored_data["id"], scored_data["anomaly_scores"], c=colors_db, s=10)
axes[0, 1].set_xlabel("id")
axes[0, 1].set_ylabel("anomaly_scores")
axes[0, 1].set_title("DBSCAN (Red = Outlier)")

# LOF
colors_lof = scored_data["lof_outlier"].map({0: "blue", 1: "red"})
axes[1, 0].scatter(scored_data["id"], scored_data["anomaly_scores"], c=colors_lof, s=10)
axes[1, 0].set_xlabel("id")
axes[1, 0].set_ylabel("anomaly_scores")
axes[1, 0].set_title("LOF (Red = Outlier)")

# KMeans
colors_kmeans = scored_data["kmeans_outlier"].map({0: "blue", 1: "red"})
axes[1, 1].scatter(scored_data["id"], scored_data["anomaly_scores"], c=colors_kmeans, s=10)
axes[1, 1].set_xlabel("id")
axes[1, 1].set_ylabel("anomaly_scores")
axes[1, 1].set_title("KMeans (Red = Outlier)")

plt.tight_layout()
plt.show()