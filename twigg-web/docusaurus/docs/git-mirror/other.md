# Other
Twigg supports mirroring to any other Git server that accepts a access with token in the URL.

**Important:**  
 * The token (or password) must have permission to **push** to the target repository.  
 * The Git server must accept authentication with credentials embedded in the URL.


**Set mirror:**
1. Go to [home](https://twigg.vc/home) and open your repository.
2. Click in repository setting icon.
    ![click_repo_setting_icon](../img/click_repo_setting_icon_img.png)
3. Scroll to `Git Mirror` section and click on the switch button to enable `Git Mirror`.
4. Construct your mirror URL using this format: 
```
https://<TOKEN>@<YOUR-GIT-SERVER>/<PATH-TO-REPO>.git
```
Example:
```
https://my-secret-token@git.mycompany.com/projects/tooling/repo.git
```
5. Click `Save URL`.

Once this URL is saved in Twigg, all submitted commits will be pushed to the specified repository’s twigg branch.