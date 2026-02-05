# GLII

**GLII** is a [WebGL](https://developer.mozilla.org/en-US/docs/Web/API/WebGL_API)
javascript abstraction library.

GLII is opinionated, and built from scratch with some specific design goals in mind:

-   **Understandability**: WebGL concepts are infamously hard to grasp; Glii
    renames some data structures and tries to make low-level data structures approachable.

-   **Object-oriented API**: [OOP](https://en.wikipedia.org/wiki/Object-oriented_programming)
    first. Glii does wrappers, inheritance, closures and factories, but does _not_ favour
    [MVC](https://en.wikipedia.org/wiki/Model%E2%80%93view%E2%80%93controller) nor reactive frameworks.

-   **Implicit context**: Instead of dragging around an instance of
    [`WebGLRenderingContext`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext)
    around, Glii wraps around it with closures. Context handling is always implicit.

-   **Avoid duplication**: Names (of attributes, uniforms, varyings, etc) should
    never be defined twice. Glii forces them to be defined just once.

-   **Do not assume 3D**: Since Glii is low-level, no 3D scene is assumed.

-   **No bundling**: Glii is ESM native, framework-free. There is no transpilation step: no webpack, no babel, no rollupJS. Stuff works directly on any
    [browsers that implement javascript modules](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Modules).

### Note from the author

The aforementioned design goals are opinionated, and they're the
ones I like, since [there are lots of things I don't like about the design of other WebGL frameworks](https://ivan.sanchezortega.es/devel/2019/02/14/a-rant-about-webgl-frameworks.html). Your preferences and requisites might not be the same as _my_ preferences
and requisites, and _that's fine_.

I shall not try to convince people that Glii is the solution to every problem, but
do consider your constraints when choosing a WebGL framework.

### Hello, world

The shortest code to do something with Glii is drawing a single vertex with
the following HTML code. Note there is no bundling whatsoever.

```html
<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8" />
	</head>
	<body>
		<canvas height="500" width="500" id="glii-canvas"> </canvas>
		<script type="importmap">
			{
				"imports": {
					"glii/": "https://unpkg.com/glii/src/"
				}
			}
		</script>
		<script type="module">
			// Point to wherever the entry point of Glii is. In this example
			// it comes from the unpkg CDN by means of the importmap defined above.
			import Glii from "glii/index.mjs";

			// Create the Glii factory. This shall wrap the context.
			const glii = new Glii(document.getElementById("glii-canvas"));

			const program = new glii.WebGL1Program({
				// The vertex shader runs only once, so it's OK to make gl_Position
				// constant at the clipspace center (0,0).
				// Since the draw mode is POINTS, gl_PointSize makes things easier to see.
				vertexShaderSource: `
void main() {
	gl_Position = vec4(0., 0., 0., 1.);
	gl_PointSize = 50.;
}`,
				// The vertex shader doesn't need to pass any data to the
				// fragment shader, so there are no varyings.
				varyings: {},

				// The fragment shader abuses the gl_PointCoord built-in variable
				// to give a bit of colour.
				fragmentShaderSource: `
void main() {
	gl_FragColor = vec4(gl_PointCoord ,0.,1.);
}`,

				// The indexBuffer tells the program how many vertices
				// there are (1) and how to interpret them (e.g. points, not triangles)
				indexBuffer: new glii.SequentialIndices({
					drawMode: glii.POINTS,
					size: 1,
				}),

				// This minimal program doesn't define any attributes,
				// textures nor uniforms.
				attributes: {},
				textures: {},
				uniforms: {},
			});

			// The program does not run automagically, and there's no implicit render loop.
			program.run();
		</script>
	</body>
</html>
```

And for those people who are non big fans of readability and like to measure things by the least lines of comment-stripped code:

```html
<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8" />
	</head>
	<body>
		<canvas height="500" width="500" id="glii-canvas"> </canvas>
		<script type="importmap"> { "imports": { "glii/": "https://unpkg.com/glii/src/" } } </script>
		<script type="module">
			import Glii from "glii/index.mjs";
			const glii = new Glii(document.getElementById("glii-canvas"));
			const program = new glii.WebGL1Program({
				vertexShaderSource: `void main() { gl_Position = vec4(0., 0., 0., 1.); gl_PointSize = 50.; }`,
				fragmentShaderSource: ` void main() { gl_FragColor = vec4(gl_PointCoord ,0.,1.); }`,
				indexBuffer: new glii.SequentialIndices({
					drawMode: glii.POINTS,
					size: 1,
				}),
			});
			program.run();
		</script>
	</body>
</html>
```

### Legalese

© Iván Sánchez Ortega <ivan@sanchezortega.es>, 2021.

Licensed under GPLv3. Yup. Complete text in the `LICENSE` file.

This repository holds 3rd-party libraries and images - see the contents of the `3rd-party` and `spec/testimages.com` directories for full info.
