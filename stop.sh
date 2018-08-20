echo "Stopping session server"
docker stop sess 2>&1 > /dev/null
docker rm sess 2>&1 > /dev/null
echo "Stopped session server"