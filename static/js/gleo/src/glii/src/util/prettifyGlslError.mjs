const errRegexp = /ERROR: 0:([0-9]+):(.*)\n/;

/**
 * Internal helper function
 *
 * Tries to parse the (first) error from a shader compile log (from a
 * getShaderInfoLog() call), extracts the corresponding (offset) line
 * from the given source, and throws an error containing that information.
 */
export default function prettifyGlslError(log, header, src, type, lineOffset) {
	const match = errRegexp.exec(log);
	if (match) {
		let lineNumber = match[1];
		const msg = match[2];

		if (lineNumber < lineOffset) {
			const line = header.split("\n")[lineNumber - 1];
			throw new Error(
				`Could not compile ${type} shader, reason:\n${msg}\nError lies in injected GLSL headers (check inputs like attributes and uniforms)\nAround line ${lineNumber}: ${line}`
			);
		} else {
			lineNumber -= lineOffset;
			const line = src.split("\n")[lineNumber - 1];
			throw new Error(
				`Could not compile ${type} shader, reason:\n${msg}\nAround line ${lineNumber}: ${line}`
			);
		}
	}

	throw new Error(`Could not compile ${type} shader, reason in unknown line:\n${log}`);
}
