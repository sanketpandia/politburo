import WebGL1Program from "./WebGL1Program.mjs";
import { registerFactory } from "../GliiFactory.mjs";

/**
 * @class MultiProgram
 * @relationship compositionOf WebGL1Program, 0..n, 1..n
 *
 * Represents a bundle of several `WebGL1Program`s. Sets their uniforms and
 * runs them all at once.
 *
 */
export default class MultiProgram {
	constructor(gl, gliiFactory, programs) {
		this._gl = gl;

		this._programs = programs || [];
	}

	/**
	 * @section
	 * @method addProgram(program: WebGL1Program): this
	 * Adds another program to the bundle.
	 */
	addProgram(program) {
		this._programs.push(program);
	}


	/**
	 * @method removeProgram(program: WebGL1Program): this
	 * Removes a program from the bundle
	 */
	removeProgram(program) {
		const i = this._programs.indexOf(program);
		if (i < 0) {
			throw new Error("Tried to remove a GL program that is not in a MultiProgram.");
		}
		this._programs.splice(i, 1);
		return this;
	}

	/**
	 * @method replaceProgram(oldProgram: WebGL1Program, newProgram: WebGL1Program): this
	 * Replaces a program, ensuring that the execution order of the rest of
	 * programs in the bundle will stay the same.
	 */
	replaceProgram(oldProgram, newProgram) {
		const i = this._programs.indexOf(oldProgram);
		if (i < 0) {
			throw new Error("Tried to remove a GL program that is not in a MultiProgram.");
		}
		this._programs.splice(i, 1, newProgram);
		return this;
	}

	/**
	 * @section Draw methods
	 * @method run():this
	 * Runs the draw call for all the bundled programs
	 * @alternative
	 * @method run(lod: Number):this
	 * Runs the draw call for all the bundled programs, passing a LoD for
	 * those programs which use a `LoDIndices`
	 * @alternative
	 * @method run(lod: String):this
	 * Idem, but for `String` LoD identifiers
	 */
	run(lod) {
		this._programs.forEach((p) => p.run(lod));
		return this;
	}

	/**
	 * @method runPartial(start: Number, count: Number):this
	 * Runs a partial draw call for all the bundled programs.
	 *
	 * This should be used only when all the bundled programs share the same `IndexBuffer`.
	 */
	runPartial(start, count) {
		this._programs.forEach((p) => p.runPartial(start, count));
		return this;
	}

	/**
	 * @section Mutation methods
	 * The following methods allow changing some of the components (uniforms/textures/bindable
	 * attributes) of the bundled programs during runtime. These change the components of all
	 * bundled programs, but will fail silently *if* a program doesn't have a given component.
	 *
	 * Note that `setTarget` is missing from the set of methods - there is no (known) use
	 * case where that would be useful, since `run()`ning all bundled programs at the same time
	 * would mean overwriting their outputs, defeating the purpose.
	 *
	 * @method setUniform(name: String, value: Number): this
	 * (Re-)sets the value of a uniform in the bundled programs, for `float`/`int` uniforms.
	 * @alternative
	 * @method setUniform(name: String, value: [Number]): this
	 * (Re-)sets the value of a uniform in the bundled programs, for `vecN`/`ivecN`/`matN` uniforms.
	 */
	setUniform(name, value) {
		this._programs.forEach((p) => {
			if (name in p._unifSetters) {
				p.setUniform(name, value);
			}
		});
		return this;
	}

	/**
	 * @method setTexture(name: String, texture: Texture): this
	 * (Re-)sets the value of a texture in the bundled programs.
	 */
	setTexture(name, texture) {
		this._programs.forEach((p) => {
			if (name in p._texs) {
				p.setTexture(name, texture);
			}
		});
		return this;
	}

	/**
	 * @method setIndexBuffer(buf: IndexBuffer): this
	 * Changes the index buffer that the bundled programs use.
	 */
	setIndexBuffer(buf) {
		this._programs.forEach((p) => p.setIndexBuffer(buf));
		return this;
	}

	/**
	 * @method setAttribute(name: Stringattr: BindableAttribute): this
	 * (Re-)sets one of the named attributes to a new `BindableAttribute`.
	 *
	 * The GLSL type of the new attribute must match the old one.
	 */
	setAttribute(name, attr) {
		this._programs.forEach((p) => {
			if (name in p._attrs) {
				p.setAttribute(name, attr);
			}
		});
		return this;
	}

	/**
	 * @section Lifetime methods
	 *
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with **all** the programs
	 * in this `MultiProgram`. Use when **none** of the programs
	 * will be used anymore.
	 */
	destroy() {
		this._programs.forEach((p) => p.destroy());
	}
}

/**
 * @factory GliiFactory.MultiProgram(programs: [WebGL1Program])
 * @class Glii
 * @section Class wrappers
 * @property MultiProgram(programs: [WebGL1Program]): Prototype of MultiProgram
 * Wrapped `MultiProgram` class
 */
registerFactory("MultiProgram", function (gl, gliiFactory) {
	return class WrappedMultiProgram extends MultiProgram {
		constructor(opts) {
			super(gl, gliiFactory, opts);
		}
	};
});
