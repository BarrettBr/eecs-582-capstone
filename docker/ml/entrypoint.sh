#!/bin/bash

# use set -e to immediately terminate if a line fails
set -e

echo "Starting ML API Service..."

# Run the commands in docker-compose
exec "$@"
