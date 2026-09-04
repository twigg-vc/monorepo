# Restore
Restores a previous version of a commit.
Twigg does this by copying the files from the selected version and creating a new commit with them 
Example:
```
 @ #1v1                               
├╯ commit 1                           
|* #1v0 --[amend]--> #1v1
├╯ commit 1                           
* #0v0 c/0v0 Submitted                 
| [Initialize]       
```
Let's restore `#1v0`, you can use any valid [commit syntax](./commit-syntax.md):
```
tw restore 1v0
```
This results in:
```
 @ #1v2                                
├╯ commit 1                            
|* #1v1 --[restore v0]--> #1v2         
├╯ commit 1                            
|* #1v0 --[amend]--> #1v1              
├╯ commit 1    
```