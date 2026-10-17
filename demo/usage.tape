# hop usage demo: hand a file to another machine, then keep organized copies in the store.
Output assets/demo-usage.gif

Require hop
Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 700
Set Padding 24
Set WindowBar Colorful
Set TypingSpeed 55ms

Hide
Type "source $DEMO_ENV laptop && hop init --endpoint http://localhost:9000 --bucket hop-demo --name laptop </dev/null >/dev/null"
Enter
Type "on web-1 && hop join $CODE --name web-1 </dev/null >/dev/null && on laptop"
Enter
Type "head -c 3145728 /dev/urandom > dump.sql && mkdir -p logs && for d in 12 13 14; do seq 1 4000 > logs/app-10-$d.log; done && clear"
Enter
Show

Type "ls"
Enter
Sleep 800ms
Type "hop push dump.sql @web-1"
Enter
Sleep 2s

Hide
Type "on web-1"
Enter
Show
Type "# on web-1:"
Enter
Sleep 400ms
Type "hop hot @"
Enter
Sleep 1.5s
Type "hop pull dump.sql @"
Enter
Sleep 2s

Hide
Type "on laptop"
Enter
Show
Type "# back on laptop: keep organized copies in web-1's store"
Enter
Sleep 400ms
Type "hop cp -r logs @web-1:backups/"
Enter
Sleep 1.5s
Type "hop ls -R @web-1:"
Enter
Sleep 1.5s
Type "hop status"
Enter
Sleep 3.5s
