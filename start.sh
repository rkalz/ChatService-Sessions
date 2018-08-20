echo "Starting sessions server"
docker run -d -it -p 8081:8080 --name=sess session go run app.go utils.go
echo "Stared sessions server"