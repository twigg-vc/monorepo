# Import an existing repository from GitHub
This guide walks you through importing an existing GitHub repository into Twigg.

Before starting, make sure you have the latest version of Twigg [installed](./tutorial/1-Installation.mdx).

Also, make sure you’ve already created an empty repository in Twigg by following [2. Create repository](./tutorial/2-create-repository.md).

1. Pull your GitHub repository locally. On your computer, navigate to the directory where you want to work and clone (or pull) your GitHub repository.

At this point, your working directory should contain the latest version of your GitHub repository.

2. Add `.twigg` to the `.gitignore` at the root of the directory - that's the folder in which Twigg stores all data.

3. Initialize Twigg (`tw init`)
Inside the repository folder, initialize Twigg:
```
tw init
```
This sets up the local Twigg metadata and prepares the directory to start tracking files with Twigg.

4. Set Twigg server (`tw server`)
Set your local repository to the Twigg repository you created earlier:
```
tw server <OWNER>/<REPO-NAME>
```
Twigg will now know which remote repository this project belongs to.

5. Verify imported files (`tw status` or `tw st`)
Run `tw status` or `tw st` bellow `changes in working directory:` you should see all files listed as created, since this is the first time Twigg is seeing them.

6. Create the import commit
```
tw commit "Importing from GitHub"
```
This commit represents the complete snapshot of your GitHub repository.

7. Set key
The server needs to know who are you when pulling and pushing. To achieve that, you need to set the CLI key.

* Go to [User settings](http://twigg.vc/user-settings)
* Scroll to the bottom and click on the `Generate New Key` button.
* Your CLI Key (tw_key_...) will be displayed. Copy it immediately. It won’t be shown again.

Use [key command](./commands/key.md) to set the CLI key.
```
tw key <YOUR CLI KEY>
```

8. Push commit to Twigg
```
tw push
```

9. Review and submit
Go to your repository page on Twigg.

You’ll see the newly pushed commit.
Review the files to make sure everything is there and looks correct.

Once everything looks good:

* Mark the commit as `[]LGTM`
* Click `Submit`

Your GitHub repository is now fully imported into Twigg.

If you want all submitted commits to be **automatically pushed** to your GitHub check [GitHub mirror section](./git-mirror/github.mdx).