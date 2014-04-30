
------------------------------------------------------------
| 				   Race conditions				  |
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


If a node realizes it is behind during Prepare phase, we shouldn't run noon. We only run no-op when server first starts.

