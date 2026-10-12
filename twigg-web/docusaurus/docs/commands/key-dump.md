# Key dump
Prints the CLI key stored for this repository, the one you set with [key](./key.md) or when [cloning](../tutorial/3-clone-repository.md):
```
tw key-dump
```
Output:
```
tw_key_...
```
This is handy for copying the key to another clone or into a CI secret. Treat the output like a password: anyone with the key can act on the server as you.
