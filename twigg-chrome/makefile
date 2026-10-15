FILES = manifest.json browser-api.js background.js golinks.js options.html options.js options.css icons

.PHONY: test package

test:
	node --test

# Both stores use the same manifest and JavaScript.
package: test
	rm -f twigg-chrome.zip
	zip -r twigg-chrome.zip $(FILES)
	cp twigg-chrome.zip twigg-firefox.zip
