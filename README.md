<h1>Project Whiteboard</h1>

<b>Description:</b>
For this project, we propose a file-sharing application specific to an academic setting. Like other file-sharing applications, it will sync a certain partition of a user’s local hard drive to our servers for easy backup and access from other machines. However, unlike a file-sharing application like Dropbox, Project Whiteboard distinguishes between different types of clients and adds file permissions accordingly. Consider the idea of a client and a super-client in the form of a student and an instructor, both in a university course setting. Both instructor and student have to manage their respective assignments; an instructor must prepare lectures, course materials, and grade student assignments, while a student works on assignments and submits them to the instructors. Up until this point, there are few products that serve as both a repository for distributing course materials and also for submitting finished assignments. Project Whiteboard will provide a solution by acting as both, making it possible for both types of users to not only provide easy backup and access from any computer, but also to view and submit their respective material without even having to open their browser.

Structure Design & Algorithms Needed for Implementation:
This project will be written in Google Go because it lends itself well to distributed systems, has relatively good documentation, and because the developers don’t want to learn a new language. 

Client
We will use a 2-layer client model. Before executing any functions, the client will check to determine whether the user is a professor or a student. The first layer will include functions specific to Project Whiteboard, such as syncing documents, adding lecture notes, submitting an assignment, etc. To prevent files shifting due to updates that were not properly synced (due to disconnection of either server or client), the client will perform infrequent checks for updates (e.g. once per day). The second layer will include more general functions which are called in the first layer. These will include (but are not limited to):
Post/Sync a document
Delete a document
Add permissions to a document
Remove permissions from a document
Add a folder
Remove a folder
Add permissions to a folder
Remove permissions from a folder
If we choose to add in Tier 3, these will also include:
Add to queue
Remove from queue (in the case the student solved his own problem)
A client will automatically connect to an arbitrary node within the consistent hashing ring (see below) that acts as the client’s buddy storage node. The client will keep note of this node’s skip list, in case it’s buddy node dies. The client will direct its queries to the buddy node (which will then redirect the request to the proper server) and rely on it to return the server in which the requested data is kept. Should this node die, detected by the client’s requests not being answered after a set period of time, the client can connect to the next node on the skip list and designate that node as its new buddy storage node.

Housekeeping Service
This is a separate client, linked either with the users computer or with the web service. If the user does not already have an account, it allows them to create one. If they do have an account, they can log in upon starting the client. The client then hashes their username and password to verify the login. Upon successful login the housekeeping client will grab from the storage servers any information they may have about the user’s files and permissions, and also checks for any missed updates. The housekeeping client then starts the other two clients needed, and passes this information along to them so that the needed updates can be made. The housekeeping client then closes.

Storage
We will have multiple storage servers to keep a backup of the data apart from the user’s own computer. We want to avoid having unintended duplicates of files and will do so by delegating files to servers based on the hash of the university class the file is associated with (e.g. the P2P networks lecture by dga is associated with 15-440). The client will remember the hash of the server it hashes to. For each user we will also store a list of files that that user has access to.

The storage servers will be decentralized, with no master node. We will implement consistent hashing as used in BitTorrent, specifically consistent hashing with skip lists. That is, each server will keep a list of the IDs of the server directly across from it on the ring, one-quarter the way across and one-eighth the way across. By not keeping a comprehensive list of every server, our system saves both space and time by not having to pass the message to as many servers. For example, if the ID the client is querying with is greater than the ID of that particular server AND the ID of the server across from it, the server will pass the job to the server across from it instead of passing it to the servers in-between. 

Storage servers will hold both file data and metadata including user credentials and file permissions. These types of data will be hashed and stored in a similar fashion as in P2, with keys formed indicating what type of data is stored at that key.  

Functions for the storage servers will include (but are not limited to):
Create User
Authenticate User
Add permissions
Remove permissions
Get skip list
Find storage node governing this hash
Rearrange data in reaction to a newly connected node
Put file into server
Get file from server
If we choose to add in Tier 3, these will also include:
Add to file queue
Pop file off queue and put (to professor)
Remove file from queue

We (may also) give the network the capacity to gain or lose storage servers during a session. This will involve mechanisms similar to what was implemented in LSP, specifically:
Storage servers should actively listen for new connections. A new server will assign itself a random ID and try to hash itself into the consistent hashing ring starting at the first node that detects it via a Listen. Using skip lists, the listening server will determine for the new server where it lies in the skip list. The predecessor server will then rehash its contents to determine which data should be moved to the new server. Also, using the skip list of the predecessor server, a skip list can be constructed for the new server. Furthermore, by using the existing skip list structure, it’s possible to skip and update every node in the ring with the new server’s information. 
Servers should ping the servers on its skip list every x number of seconds and receive a reply. Keep track of how long it has been since we didn’t get a response. If it reaches a certain time (say one epoch) and still no reply is received, declare that server disconnected and remove it from the skip list. Should this happen, all is not lost because the files that server kept still exist on the local disks of the clients (a cache, if you will). Because the client will sync infrequently, if it finds that its the server it wants to sync to is removed or has removed the class, it will copy them over to that server. Thus, it is easy to recover from a disconnected server. 

File/Directory Updates: 
Within our storage server, we will keep a map of files to both the users who have read-access and write-access to that file. Should a user with write-access edit that file, the system will send a message to all its clients indicating this change. 
If the client is not currently accessing the file, the file will be synced with its newly updated version. 
If the client has the file open, the end-user of the change will be notified and instructed to close the file to allow synchronization. If the user does not comply, the updated file will be copied to the folder but will be unsynced with the system. 
If a user tries to open the file during synchronization, he will be informed that synchronization is not finished and will be advised to wait until it is completed. Should the user open the file anyway, an additional updated un-synced version of the file will be copied to their local repository.

Testing: 
In order to thoroughly test our implementation we need to not only test by hand, but use shell scripts in combination with Go test files so that we can mimic an actual distributed system by running multiple clients and servers at once. We can do this in a manner similar to the tests provided for P1 and P2. Here is a list of the important metrics we should test for:
Create User functions
Able to authenticate user and start client
Can connect to correct server
Syncs upon Put
File permissions work as intended (student cannot access other students’ submissions)
Request is redirected to the correct node in hashing ring
Capabilities in running with multiple combinations of clients and servers (stress test)
New server finds correct spot in hashing ring
Hashing ring detects disconnected server
Client recovers from server death and redirects as expected
Correctness of level 1 client functions
Correctness of level 2 client functions
Hashing and routing correctness
Check if files/folders were synced properly
Check for correct behavior in situations with sync conflicts

Tier Implementation:
Our system can be organized into four tiers of increasing complexity. 

Tier 0 - Core Distributed system
This tier includes:
Account creation, verification, and login for both student and professor client types
Class repository creation
Saving user data on a server
Automatic file syncing between machines
Four set folder types (pre-created) with file permissions as described earlier: 
Lectures, Assignment Materials, Student Submissions, and Other Course Material
Ability to sync multiple machines to the same account
Implement consistent hashing ring that accommodates server disconnects and joins.
Make installable executable able to talk to server

Tier 1 -  Customized Syncing and Permissions
This tier includes:
Ability to change permissions of additional files within assignment directories (e.g. starter code files)
Simple interface to add file-syncing timers on certain files (e.g. professors can have a lecture calendar in which lectures will be synced to students the days they are given)
User notifications indicating file updates and repository creation
Simple interface for professor to view date and time students submitted their work


Tier 2 - Web features 
This tier includes:
View repository on web and download files individually if on unsynced-machine
Allow professors to enter grades online, and compile grades from each class a student is currently taking into one file (or online form) for students to view
Ability to share student files with other students for group projects (added names through web interface)

Tier 3 - Bonus: Online Office Hours
This tier includes:
Add “Online Office Hours” folder as a pre-made directory with assignments with permissions as described above
Allow students to add a file and comments to directory by simply dragging. Instead of instantly syncing, add file to a queue visible to the professor and sync to professor when time comes around
Add comment capability and allow professor to deem file as “reviewed” to sync back to student and remove from queue

Implementation Schedule:
Tuesday 11/27 
Should have implemented both layer 1 and layer 2 of the client since this is similar to P2 it should not take much time

Thursday 11/29 
Should have some of storage server implemented, enough so the system can run properly with post/get but not actually syncing automagically, nor dealing with a changing number of servers

Saturday 12/1
Be able to have client’s computer talk to server system and sync automagically

Sunday 12/2 
Begin writing tests and actually testing
Tier 0 should be finished/nearly finished at this time

Tuesday 12/4
Have an online and desktop interface that makes the system usable by professors and students without calling command line args, continue testing
If Tier 0 is finished already, Tier 1 could feasibly be finished by this time as well 

Thursday 12/6
Hopefully finished debugging and testing by this time, as project is due. If testing is completed early may work on adding some Tier 2 features for added oomph.