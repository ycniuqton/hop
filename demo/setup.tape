# hop setup demo: interactive init + invite on server-1, join on server-2. Render with ./demo/record.sh
Output assets/demo-setup.gif

Require hop
Require tmux
Set Shell "bash"
Set FontSize 17
Set Width 1400
Set Height 640
Set Padding 16
Set WindowBar Colorful
Set TypingSpeed 55ms

Hide
Type "$DEMO_DIR/split.sh fresh"
Enter
Wait+Screen@30s /server-2 \$/
Sleep 500ms
Show

Sleep 600ms
Type "# a local S3 server stands in for your bucket"
Enter
Sleep 600ms
Type "hop init --name server-1"
Sleep 400ms
Enter
Sleep 1.5s
Down
Sleep 600ms
Down
Sleep 600ms
Up
Sleep 600ms
Down
Sleep 900ms
Enter
Sleep 600ms
Type "http://localhost:9000"
Enter
Sleep 400ms
Type "hop-demo"
Enter
Sleep 700ms
Enter
Sleep 700ms
Enter
Sleep 700ms
Enter
Sleep 900ms
Type "n"
Enter
Sleep 2s
Type "hop invite"
Enter
Sleep 3s

Ctrl+B
Right
Sleep 700ms
Type "hop join "
Sleep 500ms
Ctrl+B
Type "J"
Sleep 800ms
Type " --name server-2"
Sleep 400ms
Enter
Sleep 1.2s
Type "n"
Enter
Sleep 1.5s
Type "hop whoami"
Enter
Sleep 3.5s
