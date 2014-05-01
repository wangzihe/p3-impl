-----------------------------------------------------------
|                                                         |
|                          Design                         |
|_________________________________________________________|

Our storage system implements paxos algorithm. The system 
consists of storage servers. Each storage server play 
the role of leader or acceptor during one iteration of 
paxos algorithm. Our storage system contains a fixed number
of storage servers. When servers first start, they will get 
a configuration file that contains the list of storage servers
in the network. Each server will send a ping message to each
other to make sure every server is up running. The ping message
is sent through rpc. 

During paxos algorithm, servers communicate using tcp connections.
Clients use rpc to send commit requests to storage servers. 
As for now, client will need to know a server port in order to 
contact that server directly.

Every time when server starts, it will run a noop recovery.
Since we can't differentiate whether a server has crashed 
before when it starts, server will always run a noop recovery
when it starts. It will send noop messsage to each server 
for iteration number starting with 0. If there is one response,
then the server will commit that value. If there is a majority
that don't know the value to that iteration, we stop the 
no-op recovery process.

------------------------------------------------------------
|                                                          |
| 	                  race conditions		               |
|                                                          |
------------------------------------------------------------

We use mutex to protect important variables like v_a, n_a.

One race condition worth mentioning is the following:

In our implementation, each node keeps track of its own
iteration number. Whenever the node makes a proposal, it needs
to get a consensus on the iteration number. 

Say an acceptor is waiting for a commit message from leader and
it is currently on iteration 5. It is possible that before the 
commit message reaches the acceptor, there is a client request
that triggers a Prepare phase on the acceptor. 

In this case, Prepare will make a proposal on iteration 5. However,
if the commit message reaches the acceptor right after Prepare sends
the proposal, the iteration number will be updated to 6. Then Prepare
makes a proposal on the wrong iteration. 

In order to prevent this from happening, we include a iteration number
in the commit message. The iteration number corresponds to the value
being committed. In addition, once the Prepare phase starts, it will
sends a proposal with iteration 5 to itself. Then it should notice that
there is a value that hasn't been committed. Therefore, it will send a
message back to itself indicating this uncommitted value. 

If a majority accepts this proposal on iteration 5, this means the 
majority is behind. This is fine. We can just start accepting the value
that hasn't been committed and catch up. If in the middle of Prepare 
phase or Accept phase, we receive a commit message for iteration 5. Then
we will just ignore it since we are proposing on iteration 5. 

If a majority rejects this proposal, then this means either we don't have
a high enough proposal ID or we are behind. If latter is the case, we
might need to do a no-op recovery.


_______________________________________________________________
|                                                              |
|                            Testing                           |
|______________________________________________________________|

I manually tested my project. 

First, you need to set up GOPATH.

Run 

go install github.com/cmu440/twitter-paxos/runners/srunner
go install github.com/cmu440/twitter-paxos/runners/crunner
go install github.com/cmu440/twitter-paxos/runners/crunner2


stunner runs a storage server. crunner and crunner2 both run a client program.

I tested my system with 3 servers. Manually run the following 3 servers:

"$GOPATH"/bin/srunner -rpc 9091 -msg 9094
"$GOPATH"/bin/srunner -rpc 9092 -msg 9095
"$GOPATH"/bin/srunner -rpc 9093 -msg 9096

After seeing two "ping called" messages for each server, this means that servers
are all up and ready to accept client requests.

Run the following command to start the first test:

"$GOPATH"/bin/crunner

This test runs one client and the client sequentially sends 1000 commit requests
to the first two storage servers (ones with port 9091 and 9092). For each commit
request, the client chooses a random server among the first two. 

After the client finishes running, we need to check that the commit log for each
server is the same. I have a python script to check that. You can run the following
commands to check:

python compare.py ./commit9095.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9095.txt

Note: Don't change the file name when you run the command.

The second test is to test whether a node can recovery after it crashes. Run the first
test as usual. But this time kill the 3rd server (the one with roc port 9093), run 

./cleanServer.sh 

and then run 

"$GOPATH"/bin/srunner -rpc 9093 -msg 9096

cleanServer.sh will clean the previous commit log of the killed server. Once you restart
the server, it should start recovery process and will eventually catch up.

Once the client program finishes, you should check that the three commit log files are
the same by running the following commands:

python compare.py ./commit9095.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9095.txt

The third test is to test whether the storage system can handle concurrent commit 
request. 

I tested my system with 3 servers. Manually run the following 3 servers:

"$GOPATH"/bin/srunner -rpc 9091 -msg 9094
"$GOPATH"/bin/srunner -rpc 9092 -msg 9095
"$GOPATH"/bin/srunner -rpc 9093 -msg 9096

After seeing two "ping called" messages for each server, this means that servers
are all up and ready to accept client requests.

Run the following command to start the third test:

"$GOPATH"/bin/crunner2

crunner2 will run multiple clients at the same time in different Go routines. Each
client will submit multiple commit requests. In this case, there will be 20
clients and each will submit 20 commit requests. 

After the program finishes, all commit logs should be the same. You can check by running

python compare.py ./commit9095.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9096.txt
python compare.py ./commit9094.txt ./commit9095.txt


