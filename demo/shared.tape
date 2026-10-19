# hop global space demo: @@: is one folder tree every machine reads and writes.
Output assets/demo-shared.gif

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
Type "hop cp -r docs @@:"
Enter
Sleep 2s

Ctrl+B
Right
Sleep 700ms
Type "hop ls -R @@:"
Enter
Sleep 1.8s
Type "hop cp @@:docs/deploy.md ."
Enter
Sleep 1.5s
Type "cat deploy.md"
Enter
Sleep 3.5s
