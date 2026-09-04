setup:
	npm install

package: setup
	npx @vscode/vsce package

install: package
	code --install-extension twigg-0.0.5.vsix

build-vsix: setup
	npx @vscode/vsce package

publish: setup
	npx @vscode/vsce publish