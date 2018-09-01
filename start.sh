echo "Starting sessions server"
docker run -d -it --network=datacenter --name=sess session go run app.go utils.go
echo "Stared sessions server"