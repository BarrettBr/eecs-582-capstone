'''
Artifact: Classification Methods Test Suite
Description: Implements and evaluates four common classification algorithms (Logistic Regression, Random Forest, KNN, SVM)
             on TempSample data from test_samples_suggestions.json to predict optimal action_class.
             Reports best parameters, scores, and confusion matrix visualizations for each method.
Author: Jacob Kice
Date Created: 03/29/2026
Preconditions: test_samples_suggestions.json exists and sklearn, matplotlib dependencies are installed
Postconditions: Prints best parameters, scores, and confusion matrix visualizations for each method
Possible errors: FileNotFoundError, JSONDecodeError, ValueError for missing fields, sklearn exceptions for invalid parameter ranges
Side effects: Opens matplotlib windows for confusion matrix plots
Invariants: Feature selection matches main.py (fan_on, temperature, heater_power), target is action_class
Known faults: Small sample sets may limit cross-validation reliability or classifier diversity
'''


# Standard library imports
import json

# Third-party imports
import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split, GridSearchCV
from sklearn.preprocessing import LabelEncoder
from sklearn.linear_model import LogisticRegression
from sklearn.ensemble import RandomForestClassifier
from sklearn.neighbors import KNeighborsClassifier
from sklearn.svm import SVC
from sklearn.metrics import accuracy_score, precision_score, recall_score, f1_score, confusion_matrix, classification_report
from sklearn.decomposition import PCA
import matplotlib.pyplot as plt
import seaborn as sns


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


def prepare_features_and_target(df):
    '''
    Extracts relevant features and target label for classification from sample DataFrame.
    Creates a composite target combining action_class, priority, and recommended_actions.
    Args:
        df: DataFrame of sample data.
    Returns:
        Tuple of (features DataFrame, target Series, LabelEncoder for classes).
    '''
    features = df.loc[:, ['fan_on', 'temperature', 'heater_power']].copy()
    features['fan_on'] = features['fan_on'].astype(int)
    
    # Create composite target label: action_class|priority|recommended_actions
    action_class = df.loc[:, 'action_class'].astype(str)
    priority = df.loc[:, 'priority'].astype(str)
    recommended_actions = df.loc[:, 'recommended_actions'].apply(
        lambda x: ','.join(x) if isinstance(x, list) and len(x) > 0 else 'none'
    )
    
    target = action_class + '|' + priority + '|' + recommended_actions
    
    # Encode target labels
    le = LabelEncoder()
    target_encoded = le.fit_transform(target)
    
    return features, target_encoded, le


# Logistic Regression classifier

def search_logistic_regression(X_train, X_test, y_train, y_test):
    '''
    Runs Logistic Regression with hyperparameter tuning and selects the best model by accuracy.
    Args:
        X_train: Training features.
        X_test: Testing features.
        y_train: Training target labels.
        y_test: Testing target labels.
    Returns:
        Best model found and evaluation metrics dictionary.
    '''
    param_grid = {
        'C': [0.001, 0.01, 0.1, 1, 10],
        'solver': ['lbfgs', 'liblinear'],
        'max_iter': [100, 200, 500]
    }
    
    grid = GridSearchCV(LogisticRegression(random_state=42), param_grid, cv=3, scoring='accuracy')
    grid.fit(X_train, y_train)
    
    best_model = grid.best_estimator_
    y_pred = best_model.predict(X_test)
    
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred, average='weighted', zero_division=0),
        'recall': recall_score(y_test, y_pred, average='weighted', zero_division=0),
        'f1': f1_score(y_test, y_pred, average='weighted', zero_division=0)
    }
    
    print(f'Logistic Regression best params: {grid.best_params_}')
    print(f'Logistic Regression Accuracy: {metrics["accuracy"]:.3f}')
    print(f'Logistic Regression Precision: {metrics["precision"]:.3f}')
    print(f'Logistic Regression Recall: {metrics["recall"]:.3f}')
    print(f'Logistic Regression F1-Score: {metrics["f1"]:.3f}')
    
    return best_model, y_pred, metrics


# Random Forest classifier

def search_random_forest(X_train, X_test, y_train, y_test):
    '''
    Runs Random Forest with hyperparameter tuning and selects the best model by accuracy.
    Args:
        X_train: Training features.
        X_test: Testing features.
        y_train: Training target labels.
        y_test: Testing target labels.
    Returns:
        Best model found and evaluation metrics dictionary.
    '''
    param_grid = {
        'n_estimators': [10, 50, 100],
        'max_depth': [3, 5, 10, None],
        'min_samples_split': [2, 5, 10]
    }
    
    grid = GridSearchCV(RandomForestClassifier(random_state=42), param_grid, cv=3, scoring='accuracy')
    grid.fit(X_train, y_train)
    
    best_model = grid.best_estimator_
    y_pred = best_model.predict(X_test)
    
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred, average='weighted', zero_division=0),
        'recall': recall_score(y_test, y_pred, average='weighted', zero_division=0),
        'f1': f1_score(y_test, y_pred, average='weighted', zero_division=0)
    }
    
    print(f'Random Forest best params: {grid.best_params_}')
    print(f'Random Forest Accuracy: {metrics["accuracy"]:.3f}')
    print(f'Random Forest Precision: {metrics["precision"]:.3f}')
    print(f'Random Forest Recall: {metrics["recall"]:.3f}')
    print(f'Random Forest F1-Score: {metrics["f1"]:.3f}')
    
    return best_model, y_pred, metrics


# K-Nearest Neighbors classifier

def search_knn(X_train, X_test, y_train, y_test):
    '''
    Runs K-Nearest Neighbors with hyperparameter tuning and selects the best model by accuracy.
    Args:
        X_train: Training features.
        X_test: Testing features.
        y_train: Training target labels.
        y_test: Testing target labels.
    Returns:
        Best model found and evaluation metrics dictionary.
    '''
    param_grid = {
        'n_neighbors': [3, 5, 7, 9],
        'weights': ['uniform', 'distance'],
        'algorithm': ['auto', 'ball_tree', 'kd_tree']
    }
    
    grid = GridSearchCV(KNeighborsClassifier(), param_grid, cv=3, scoring='accuracy')
    grid.fit(X_train, y_train)
    
    best_model = grid.best_estimator_
    y_pred = best_model.predict(X_test)
    
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred, average='weighted', zero_division=0),
        'recall': recall_score(y_test, y_pred, average='weighted', zero_division=0),
        'f1': f1_score(y_test, y_pred, average='weighted', zero_division=0)
    }
    
    print(f'KNN best params: {grid.best_params_}')
    print(f'KNN Accuracy: {metrics["accuracy"]:.3f}')
    print(f'KNN Precision: {metrics["precision"]:.3f}')
    print(f'KNN Recall: {metrics["recall"]:.3f}')
    print(f'KNN F1-Score: {metrics["f1"]:.3f}')
    
    return best_model, y_pred, metrics


# Support Vector Machine classifier

def search_svm(X_train, X_test, y_train, y_test):
    '''
    Runs Support Vector Machine with hyperparameter tuning and selects the best model by accuracy.
    Args:
        X_train: Training features.
        X_test: Testing features.
        y_train: Training target labels.
        y_test: Testing target labels.
    Returns:
        Best model found and evaluation metrics dictionary.
    '''
    param_grid = {
        'C': [0.1, 1, 10, 100],
        'kernel': ['linear', 'rbf', 'poly'],
        'gamma': ['scale', 'auto']
    }
    
    grid = GridSearchCV(SVC(random_state=42), param_grid, cv=3, scoring='accuracy')
    grid.fit(X_train, y_train)
    
    best_model = grid.best_estimator_
    y_pred = best_model.predict(X_test)
    
    metrics = {
        'accuracy': accuracy_score(y_test, y_pred),
        'precision': precision_score(y_test, y_pred, average='weighted', zero_division=0),
        'recall': recall_score(y_test, y_pred, average='weighted', zero_division=0),
        'f1': f1_score(y_test, y_pred, average='weighted', zero_division=0)
    }
    
    print(f'SVM best params: {grid.best_params_}')
    print(f'SVM Accuracy: {metrics["accuracy"]:.3f}')
    print(f'SVM Precision: {metrics["precision"]:.3f}')
    print(f'SVM Recall: {metrics["recall"]:.3f}')
    print(f'SVM F1-Score: {metrics["f1"]:.3f}')
    
    return best_model, y_pred, metrics


# Evaluation metrics

def evaluate_classification(y_test, y_pred, method, label_encoder):
    '''
    Computes and prints detailed classification evaluation metrics.
    Args:
        y_test: Ground truth labels.
        y_pred: Predicted labels.
        method: String name of classification method.
        label_encoder: LabelEncoder for decoding class names.
    Returns:
        None.
    '''
    print(f'\n{method} Classification Report:')
    class_names = label_encoder.classes_
    print(classification_report(y_test, y_pred, target_names=class_names, zero_division=0))


# Visualization

def visualize_confusion_matrix(y_test, y_pred, method, label_encoder):
    '''
    Generates and displays a confusion matrix for the given predictions.
    Args:
        y_test: Ground truth labels.
        y_pred: Predicted labels.
        method: String name of classification method.
        label_encoder: LabelEncoder for decoding class names.
    Returns:
        None.
    '''
    cm = confusion_matrix(y_test, y_pred)
    class_names = label_encoder.classes_
    
    plt.figure(figsize=(8, 6))
    sns.heatmap(cm, annot=True, fmt='d', cmap='Blues', 
                xticklabels=class_names, yticklabels=class_names)
    plt.title(f'{method} Confusion Matrix')
    plt.xlabel('Predicted')
    plt.ylabel('Actual')
    plt.tight_layout()
    plt.show()


# Main test runner

def main():
    '''
    Main entry point: loads data, splits into train/test sets, runs parameter search for each classification method,
    prints and visualizes results, and compares overall performance.
    Args:
        None.
    Returns:
        None.
    '''
    # Load and prepare data
    df = load_samples('ml\\app\\test_samples_suggestions.json')
    X, y, le = prepare_features_and_target(df)
    
    print('Features used for classification:')
    print(X)
    print(f'\nTarget: Composite label (action_class|priority|recommended_actions)')
    print(f'Unique composite classes: {len(le.classes_)}')
    print(f'Class mapping:')
    for idx, label in enumerate(le.classes_):
        print(f'  {idx}: {label}')
    print(f'\nClass distribution:\n{pd.Series(y).value_counts().sort_index()}')
    
    # Split data into train/test sets
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.3, random_state=42, stratify=y)
    print(f'\nTrain set size: {len(X_train)}, Test set size: {len(X_test)}')
    
    # Store results for comparison
    results = {}
    
    # Test each classification method
    print('\n--- Logistic Regression ---')
    lr_model, lr_pred, lr_metrics = search_logistic_regression(X_train, X_test, y_train, y_test)
    evaluate_classification(y_test, lr_pred, 'Logistic Regression', le)
    visualize_confusion_matrix(y_test, lr_pred, 'Logistic Regression', le)
    results['Logistic Regression'] = lr_metrics
    
    print('\n--- Random Forest ---')
    rf_model, rf_pred, rf_metrics = search_random_forest(X_train, X_test, y_train, y_test)
    evaluate_classification(y_test, rf_pred, 'Random Forest', le)
    visualize_confusion_matrix(y_test, rf_pred, 'Random Forest', le)
    results['Random Forest'] = rf_metrics
    
    print('\n--- K-Nearest Neighbors ---')
    knn_model, knn_pred, knn_metrics = search_knn(X_train, X_test, y_train, y_test)
    evaluate_classification(y_test, knn_pred, 'KNN', le)
    visualize_confusion_matrix(y_test, knn_pred, 'KNN', le)
    results['KNN'] = knn_metrics
    
    print('\n--- Support Vector Machine ---')
    svm_model, svm_pred, svm_metrics = search_svm(X_train, X_test, y_train, y_test)
    evaluate_classification(y_test, svm_pred, 'SVM', le)
    visualize_confusion_matrix(y_test, svm_pred, 'SVM', le)
    results['SVM'] = svm_metrics
    
    # Print comparison summary
    print('\n=== Performance Comparison ===')
    results_df = pd.DataFrame(results).T
    print(results_df.round(3))
    
    best_method = results_df['f1'].idxmax()
    print(f'\nBest overall method (by F1-Score): {best_method}')


if __name__ == '__main__':
    main()
