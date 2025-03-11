#!/bin/sh
# Start the Go backend (assumed to listen on port 8080)
echo "Starting Go backend..."
/bin/squad-aegis &

# Start the Nuxt 3 server (assumed to listen on port 3000)
echo "Starting Nuxt 3 server..."
# Adjust the command if your Nuxt start command is different
node /app/web/.output/server/index.mjs &

# Start Nginx (in the foreground)
echo "Starting Nginx reverse proxy..."
nginx -g 'daemon off;'
