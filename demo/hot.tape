# hop hot-shelf demo: server-1 pushes a file by name, server-2 pulls it by name. Render with ./demo/record.sh
Output assets/demo-hot.gif

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
Type "ls"
Enter
Sleep 900ms
Type "hop push dump.sql"
Enter
Sleep 2s

Ctrl+B
Right
Sleep 700ms
Type "hop hot"
Enter
Sleep 1.8s
Type "hop pull dump.sql"
Enter
Sleep 1.5s
Type "ls -sh dump.sql"
Enter
Sleep 3.5s
