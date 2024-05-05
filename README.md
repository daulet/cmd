
# cmd

Most common usage is pass in your message as an argument to the command:
```bash
$ cmd hi
Hello! How can I help you today?
```

## run code

If you'd like to run the generated shell command or script, use `-run`:
```bash
$ cmd -run print third last commit hash
To print the third last commit hash, you can use the following Git command:

bash
git log --pretty=format:"%H" -n 1 --skip 2

This command will display the commit hash of the commit that is two commits before the most recent one. The `--pretty=format:"%H"` option specifies that you want to display the commit hash in the output, and the `-n 1` option limits the output to only one commit. The `--skip 2` option skips the two most recent commits and displays the hash of the third last commit.
a200e6d429e2888344d7254ac02a00618ab432a2
```

## execute code

To _just_ execute generated command or a script (like `-run`), but _without_ actually outputing it (useful for piping), use `-exec`, which will not output generation hence be patient:
```bash
$ cmd -exec print shell command to brief description for last five commits
27a6a07 add an option to execute generate command
ac88d6a parse code blocks as we stream, not after the fact
a200e6d simplify now that code parsing is async
6d36937 rename Buffer to Code
d42893c (HEAD -> main, origin/main) simplify code parser, make exec truly optional
```

## chat

To start a chat session use `-chat`:
```bash
$ cmd -chat
User> (your message)
```

which is also compatible with other flags, like `-run`:
```bash
$ cmd -run -chat
User > 
```
