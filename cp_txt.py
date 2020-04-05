import os

from config import SCP_STRING

HTML_DOC= """
<!doctype html>
<html>
  <head>
    <link rel="stylesheet" href="/main.css">
    <title>Stream of Consciousness</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
  </head>
  <body>
  <div class="container">
    <pre>STREAM\n\n{}</pre>
  </div>
  </body>
</html>
"""

with open('cj.txt', 'r') as forward_cj:
    fwd = forward_cj.read()
    rev_split = fwd.split("\n\n")
    rev_split.reverse()

with open('cj_rev.txt', 'w') as rev_cj:
    # some hack for the first line of the page
    rev = rev_split[0] + '\n' + '\n\n'.join(rev_split[1:])
    rev_cj.write(rev)

with open('index.html', 'w') as html_cj:
    html_string = HTML_DOC.format(rev)
    html_cj.write(html_string)

os.system(SCP_STRING)
