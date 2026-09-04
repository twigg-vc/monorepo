# Load
Loads the files from a commit into your working directory - without creating a new commit.

Example:
```
 @ #2v0               
├╯ commit 2           
|* #1v0               
├╯ create example.txt 
* #0v0 c/0v0 Submitted
| [Initialize]      
```

To load the files from `#1v0`, run:
```
tw load 1v0
```
you'll see:
```
loaded commit #1v0 into the working directory
```
By running `tw st` or `tw status` you can see the changes:
```
changes in working directory:
example.txt: created

current commit tree:
 @ #2v0               
├╯ commit 2           
|* #1v0               
├╯ Create example.txt 
* #0v0 c/0v0 Submitted
| [Initialize] 
```
Your working directory now contains all files from `#1v0`, but no new commit was created.
