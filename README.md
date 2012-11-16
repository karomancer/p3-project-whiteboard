P3 
a.k.a. KitchenSync

DEFINITIONS
- Folder syncing across clients. Clients have same priveleges to repository
- One client is "special" (e.g. a Professor "special" client and "student" clients) and has different access

ROLES
"Special" Client
  Can 
  - see all clients work in the submission repository
  - 
  Cannot
  - see client work without designation
  - 
  - Can create repositories

Client
  Can
  - create private folders within working directory and can designate as shared directories (groupwork? Partial credit?)
  - flag work for submission (automatically submits the date and time it's due). 
    |-> Doing so registers file in queue as "will be submitted on time" 
  Cannot
  - create main repositories
  - see other clients work (unless designated)
  - see final deliverable contents of other clients


INITIAL FEATURE SET
- Simple UI to designate folders to have different purposes
  Purposes: 
    - Common material folder (e.g. Lectures)
    - Working directory (e.g. Homework folders)
      |-> Final deliverable folder (e.g. Assignment Submissions)
- Queued review 
  Students can queue a file or a portion of a file (?) to be reviewed by a TA or professor
  |-> can take it off the queue at any time (if they figure it out)
  |-> A FIFO queue is kept and is visible to professors and TAs, which they can attend to at their own leisure
  |-> Each thing on the queue can be put there with a description (line numbers, context, etc)
  |-> Not just for code! Essays that need review, artwork that needs advice, etc


IMPLEMENTATION
Multiple Access: 
  - Leases? 
    |-> Since collaboration likely won't be a strong feature, this might work well.
  - Dropbox style?
    |-> It's all a question of collaboration.
Review Queue:
  - 

