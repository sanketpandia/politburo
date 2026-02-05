/**
 * This is just a naïve poor substitute for the "CSS module imports"
 * feature: https://chromestatus.com/feature/5948572598009856
 *
 * It should be replaced with that, browser support permitting.
 *
 * One of the design intentions of Gleo is to not need a build system, so
 * depending on anything that bundles CSS together is a no-go.
 */

const head = document.getElementsByTagName("head")[0];
const el = document.createElement("style");
el.type = "text/css";
head.appendChild(el);

export default function css(str) {
	const styleNode = document.createTextNode(str);
	el.appendChild(styleNode);
}
