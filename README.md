
# cmd

Most common usage is to pass your message as an argument (wrap it in quotes if it contains special shell modifiers like `*` or `?`): `cmd hi` or `cmd "what's up?"`. Addtionally, you can pipe content to it:
```bash
cat README.md | cmd briefly describe functionality
Cmd is a versatile command-line tool that leverages AI to understand natural language input and generate shell commands or even entire scripts. It offers various flags to enhance its functionality. You can use `-run` to execute the generated command and display the output, while `-exec` executes the command without showing the generation process or output. For multi-turn interactions, there's `-chat`, and you can configure the model, connectors, and other settings with `-config` and related flags. Additionally, content can be piped into cmd for quick processing.
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

To start a multi turn chat session use `-chat`:
```bash
$ cmd -chat
User> (your message)
```

Which is also compatible with other flags, like `-run`, that can be used to iterate on a solution:
```bash
$ cmd -run -chat
User> html for a bouncing ball
```

## Configure

You can check current configuration using `cmd -config`, and to change it use:
* `-model` to set the model (use `-list-models` to see your options);
* `-connectors` to set comma delimited connectors (use `-list-connectors` to see your options);
* `-temperature` to set the temperature;
* `-top-p` to set the top P;
* `-top-k` to set the top K;
* `-frequency-penalty` to set the frequency penalty;
* `-presence-penalty` to set the presence penalty;
```bash
$ cmd -connectors web-search
```
