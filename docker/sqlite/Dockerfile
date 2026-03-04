# Use a lightweight image, pinned to latest version as of 3/4/26
FROM alpine:3.2.3
 
# Install SQLite
RUN apk update && apk add sqlite
 
# Set the default working directory
WORKDIR /db
 
# Set the default command to open SQLite
CMD ["sqlite3"]
