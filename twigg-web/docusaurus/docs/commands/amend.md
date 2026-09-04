# Amend
To edit a commit just goto it and start editing. You can check your changes with `tw st`, or use the [VS code extension](../vs-code-extension.md) for a more detailed comparison.
```
changes in working directory:
example.txt: modified

current commit tree:
@ #1v0
| commit 1
.
.
.
```
Once you finished, just run:
```
tw amend <optional new commit title>
```
If you provide a title, it replaces the old one.

You can see (`tw -a`) how `amend` just added a new commit version and you can still [restore](./restore.md) previous ones.
```
 @ #1v1                               
├╯ commit 1                           
|* #1v0 --[amend]--> #1v1
├╯ commit 1                           
* #0v0 c/0v0 Submitted                 
| [Initialize]       
```
Keep in mind that all descendants commits are automatically rebased to the new version as well.

### I accidentally amended in a commit
Don’t worry - with Twigg you never lose your work. Let’s just go back to where you were. Let's say you wanted to create a new commit on top of the current one, but you accidentally amended the current one resulting in:
```
 @ #1v1                               
├╯ commit 1                           
|* #1v0 --[amend]--> #1v1
├╯ commit 1                           
* #0v0 c/0v0 Submitted                 
| [Initialize]       
```
To fix this mistake, just [restore](./restore.md) `#1v0` resulting in:
```
 @ #1v2                                
├╯ commit 1                            
|* #1v1 --[restore v0]--> #1v2         
├╯ commit 1                            
|* #1v0 --[amend]--> #1v1              
├╯ commit 1    
```
Now we can simply [load](./load.md) `#1v1`:
```
tw load 1v1
```
Now you’ve loaded all the files from `#1v1` to the working directory.
Now simply run `tw commit <title>` to create a new commit.