# hop setup demo: interactive init on one machine, join on a second. Render with ./demo/record.sh
Output assets/demo-setup.gif

Require hop
Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 680
Set Padding 24
Set WindowBar Colorful
Set TypingSpeed 55ms

Hide
Type "source $DEMO_ENV laptop && clear"
Enter
Show

Type "# demo: a local S3-compatible server stands in for your bucket"
Enter
Sleep 600ms
Type "hop init --name laptop"
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
Sleep 2.5s

Type "hop invite"
Enter
Sleep 3s

Hide
Type "on web-1"
Enter
Show
Type "# meanwhile, on a second machine:"
Enter
Sleep 500ms
Type "hop join $CODE --name web-1"
Enter
Sleep 1.2s
Type "n"
Enter
Sleep 3s
