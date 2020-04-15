REPLICAS=$1

for (( c=0; c<${REPLICAS}; c++ ))
do
    docker run -v $(pwd)/:/client --name 1mclient_$c -d alpine .//app/main
done


# ./setup.sh 20000 50 172.17.0.1