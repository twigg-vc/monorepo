# Resolving conflicts
When performing commands like rebase [rebase](./commands/rebase.md), [amend](./commands/amend.md) or [restore](./commands/restore.md), conflicts can occur. When this happens, Twigg automatically creates a new version of the commit and marks where the conflict happened.
```
<<<<<<<<< [commit id and version]
target
=========
source
>>>>>>>>> [commit id and version]
```  
Now you have a commit with conflict. If you run `tw st` or `tw status`:
```
conflicts:
example.txt has unresolved conflicts
changes in working directory:
no changes

current commit tree:
@ #3v1 Conflicts
| commit 3      
* #2v1          
| commit 2      
```
So in this case commit `3` has a conflict in file `example.txt`. 
By opening `example.txt` you'd see:
```example.txt
<<<<<<<<< #3v0
I just write incredible code.
=========
I sometimes write incredible code.
>>>>>>>>> #2v1
```
Twigg shows both versions of the line that conflicted - the upper section from the target commit, and the lower one from the source.
Now, you just need to edit the file to reflect the desired result.
In this case, the correct version is pretty clear:
```example.txt
<<<<<<<<< #3v0 [DELETE]
I just write incredible code. [KEEP]
========= [DELETE]
I sometimes write incredible code. [DELETE]
>>>>>>>>> #2v1 [DELETE]
```
Resulting in:
```example.txt
I just write incredible code.
```
Save the file, and then run `tw st` or `tw status` again:
```
conflicts:
example.txt conflicts resolved :)
changes in working directory:
example.txt: modified

current commit tree:
@ #3v1 Conflicts
| commit 3      
* #2v1          
| commit 2      
```
All conflicts are now resolved.
To finish, simply [amend](./commands/amend.md) the commit, and that’s it, conflict resolved! 

You still can see the previous version with `tw -a`:
```
 @ #3v2                               
├╯ commit 3                           
|* #3v1 [Conflicts, --[amend]--> #3v2]
├╯ commit 3                           
* #2v1                                
| commit 2  
```
That means you can:
 * Go back and review how you resolved it
 * [Restore](./commands/restore.md) it if you want to resolve it differently
 * Push it to the server and ask someone to review your resolution
 * Push it with the conflict for another person to resolve if you’re feeling like too much of a vibe coder
