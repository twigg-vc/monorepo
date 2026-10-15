FILES = manifest.json background.js golinks.js options.html options.js options.css icons

test:
	node --test

# Zip to upload to the Chrome Web Store.
package: test
	rm -f twigg-chrome.zip
	zip -r twigg-chrome.zip $(FILES)
