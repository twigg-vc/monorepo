import DocCardList from '@theme/DocCardList';

# Git Mirror
A Git Mirror allows Twigg to **automatically push submitted commits** to the `twigg` branch of an external Git repository.

This is especially useful when you're **gradually migrating a project to Twigg but still rely on your existing Git server for deployments**, **CI pipelines**, or other automation. With a Git Mirror configured, code review and workflow happen in Twigg, while your Git server continues to receive the latest state of the repository exactly as before.

<DocCardList />