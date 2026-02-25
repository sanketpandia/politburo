// Template regexp and function straight from Leaflet

// @namespace Util
// @function template(str: String, data: Object): String
// Simple templating facility, accepts a template string of the form `'Hello {a}, {b}'`
// and a data object like `{a: 'foo', b: 'bar'}`, returns evaluated string
// `('Hello foo, bar')`. You can also specify functions instead of strings for
// data values — they will be evaluated passing `data` as an argument.

const templateRe = /\{ *([\w_ -]+) *\}/g;

export default function template(str, data) {
	return str.replace(templateRe, function (str, key) {
		var value = data[key];

		if (value === undefined) {
			throw new Error("No value provided for variable " + str);
		} else if (typeof value === "function") {
			value = value(data);
		}
		return value;
	});
}
