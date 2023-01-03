# examSubmission
To run the program, open 2 terminals, both having 2 split terminals. 
In 1st split terminals run 
- go run Replica.go 0 true
- go run Replica.go 1 false
to create Primary Replica and Replica,
and in 2nd split terminals run 
- go run client.go 1
- go run client.go 2
to create 2 clients