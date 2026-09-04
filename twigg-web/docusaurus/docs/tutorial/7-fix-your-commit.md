# 7. Fix your commit 
Make the requested changes in your file.
```
echo "Like, dangerously good." >> example.txt
```
Amend the commit
```
tw amend
```
You should see: `ammended commit #1v0 c/1v0 -> #1v1 c/1`

You just created a new version of your commit. Every time you run `amend`, `rebase`, or `restore`, you’re creating a new version of your commit. To check all versions, run:
```
tw -a
```
You should see output:
```
 @ #1v1 c/1                              
├╯ My first commit!                      
|* #1v0 c/1v0 [Pushed, --[amend]--> #1v1]
├╯ My first commit!                      
* #0v0 c/0v0 Submitted                   
| [Initialize]                           
```
Now push this version to the server.
```
push
```
If you run `tw` again, you will see:
```
@ #1v1 c/1v1 Pushed
| My first commit!
* #0 c/0v0 Submitted
| [Initialize]
```

The server just created a new commit version: `c/1v1`

<kbd>CTRL</kbd>+<kbd>Click</kbd> `c/1v1` to open the commit in Twigg.
