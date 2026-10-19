# hop store demo: server-1 files organized copies into server-2's store; server-2 lists and fetches them.
Output assets/demo-store.gif

Require hop
Require tmux
Set Shell "bash"
Set FontSize 17
Set Width 1400
Set Height 420
Set Padding 16
Set WindowBar Colorful
Set TypingSpeed 55ms

Hide
Type "$DEMO_DIR/split.sh ready"
Enter
Wait+Screen@30s /server-2 \$/
Sleep 500ms
Show

Sleep 600ms
Type "hop cp -r logs @server-2:backups/"
Enter
Sleep 2s

Ctrl+B
Right
Sleep 700ms
Type "hop ls -R @:"
Enter
Sleep 1.8s
Type "hop cp -r @:backups/logs ."
Enter
Sleep 1.5s
Type "ls logs"
Enter
Sleep 3.5s
