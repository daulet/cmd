
Most common usage is pass in your message as an argument to the command:
```bash
$ cmd hi
Hello! How can I help you today?
```

If you'd like to execute generated shell command or script, use `-exec`:
```bash
$ cmd -exec output shell command for git command to list last ten commits
* cc52be7 (HEAD -> main, origin/main) factor out client
* 715eb25 extract chat functionality as an option
* 467a606 add support of rendering web pages
* 05272c7 create friendly stream reader
* 9936700 factor out os package and use standard reader and writer
* ce6e32b stream messages
* 1b28583 interactive chat
* b0dfc1d just chat
```

To start a chat session use `-chat`:
```bash
$ cmd -chat
User> (your message)
```
