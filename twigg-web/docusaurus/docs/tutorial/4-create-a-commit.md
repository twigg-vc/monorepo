# 4. Create a commit

You just cloned your repository! Now let's start the fun.

## What is a commit?
A commit can be understood as snapshot of all the files in the repository.

## See your current commit
Run `tw` to see your current commit:
```
@ #0v0 c/0v0 Submitted
| [Initialize]
```
Lets unpack what everything means:
 * `@` your current commit (others will appear soon)
 * `#0v0` version 0 of the commit with *local id* 0 (more on that later)
 * `c/0v0` version 0 of the commit with *server id* 0 (more on that later)
 * `Submitted` indicates this commit is submitted
 * `[Initialize]` a short title describing the commit. This one in particular was auto-generated, because the first commit is created automatically with the repository.

## Commit Ids?

There are 2 type of commit Ids: `local` and `server`.
 * The *local id* (represented as `#<local id>` or `<local id>`) is a number that identifies a commit on someone's computer.
That number starts at 0 and increases by 1 for each commit created on that computer.
 * The *server id* (represented as `c/<server id>` or `c<server id>`) is also a number that starts
at 0 and is increased by 1 for every commit created on the server or pushed to it.

Thus, if two users create two commits on their machines, those commits will have
*local id* `#1` and `#2` on their machines. Once they push to the server, those commits
will get a *server id*. Note that the server Id **globally** identifies a commit.

## Commit versions?

All commits have an immutable version. When a commit is created, it has version `0`.
When that commit is **modified** (amended, restored, rebased or submitted - more on that later),
that version is increased by 1.

## Submitted?

When commits are first created, they are considered work in progress.
This means you can still edit them.

Once a commit is submitted, it is considered **final** (immutable) and
part of the main version history of the repository.

Note that submitting a commit also increases it's version.

## Create a commit

Let’s create our first commit!

Create an example file by running the following command:
```
echo "I just write incredible code." > example.txt
``` 
Run the following command to create a commit
```
tw commit "My first commit"
```
Run `tw` to see the commit you just created on top of `#0v0`:
```
@ #1v0
| My first commit!
* #0v0 c/0v0 Submitted
| [Initialize]
```
