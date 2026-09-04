# Rebase
Creates a new version of a commit by changing its parent to a new target. All of its descendant commits are automatically rebased to the new version as well.

Syntax:
 * `source` commit to be rebased, use valid [commit syntax](./commit-syntax.md).
 * `target` commit to be used as the new parent, use valid [commit syntax](./commit-syntax.md).
```
tw rebase <source> <target>
``` 
If you only provide the `target`, the `source` will default to your current commit (`@`):
```
tw rebase <target>
``` 

Example:
```
 * #3v0               
 | commit 3           
 * #2v0               
├╯ commit 2           
|@ #1v0               
├╯ commit 1           
* #0v0 c/0v0 Submitted
| [Initialize] 
```
Let's rebase `#2v0` in `#1v0`:
```
tw rebase 2 1
```
If you run tw again, you will see:
```
* #3v1                
| commit 3            
* #2v1     
| commit 2            
@ #1v0                
| commit 1            
* #0v0 c/0v0 Submitted
| [Initialize]   
```
Commits `#2` and `#3` were successfully rebased, both now include the changes from commit `#1`.

Notice how their versions increased.
If you run `tw -a` to view all versions, you can see their history and how parents changed:
```
 * #3v0 --[auto rebase]--> #3v1
 | commit 3                    
 * #2v0 --[rebase]--> #2v1     
├╯ commit 2                    
|* #3v1                        
|| commit 3                    
|* #2v1                        
|| commit 2                    
|@ #1v0                        
├╯ commit 1                    
* #0v0 c/0v0 Submitted         
| [Initialize]   
```
Commit `#3` shows as `auto rebase` since its parent (`#2`) was rebased.

## Got conflict rebasing
When you rebase - whether directly or through automatic child rebases - conflicts can occur.

Unlike most version controls, Twigg actually creates commits that contain conflicts instead of stopping the process.
Commits with conflicts are mostly like any other commit, except some actions (like rebase or submit) are limited until you amend to resolve them. See [Resolving conflicts](../resolving-conflicts.md) for more details.

To check which files have solved/unsolved conflicts, run `tw status` (or the short version `tw st`)

Resolve the conflicts and run `tw amend` to create a new version without conflicts.

You can the run the following to continue rebasing the descendants:
```
tw rebase <commit was supposed to rebase next> <commit that had conflict>
```