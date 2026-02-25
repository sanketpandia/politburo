import Acetate from "../acetates/Acetate.mjs";
import { VectorField } from "./Field.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import glslVecNify from "../util/glslVecNify.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class ParticleSimulator
 * @inherits VectorField
 *
 * A `VectorField` which displays moving particles on it. The movement of the
 * particles depends on the direction and strength of the vector field.
 *
 */

export default class ParticleSimulator extends VectorField {
	#particleCount;
	#speedMultiplier;
	#minOpaqueSpeed;
	#particleColour;

	#partTex1;
	#partFB1;
	#partTex2;
	#partFB2;

	#texBytes;

	#respawnInterval;

	/**
	 * @constructor ArrowHeadField(target: GliiFactory, opts?: QuadBin Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option particles: Number = 256
			 * The amount of particles to render.
			 */
			particles = 256,

			/**
			 * @option speedMultiplier: Number = 1
			 * The default speed is the field value in CSS pixels per second.
			 *
			 * This option multiplies the speed by the given value.
			 */
			speedMultiplier = 1,

			/**
			 * @option particleLifetime: Number = 10
			 * Lifetime of the particles, in seconds.
			 *
			 */
			particleLifetime = 10,

			/**
			 * @option minOpaqueSpeed: Number = 2
			 * Minimum speed for a particle to become opaque. If the speed
			 * of a particle is below this, it'll be semitransparent.
			 *
			 * The "speed of a particle" in this context is the value of the
			 * underlying vector field times `speedMultiplier`.
			 *
			 * Setting this to zero will make all particles opaque.
			 */
			minOpaqueSpeed = 2,

			/**
			 * @option particleColour: Colour = "black"
			 * Desired colour of the particles.
			 *
			 * This is the colour they take when their speed is over `minOpaqueSpeed`.
			 */
			particleColour = [0, 0, 0, 1],

			...opts
		} = {}
	) {
		super(target, opts);
		const glii = this.glii;

		this.#particleCount = particles;
		this.#speedMultiplier = speedMultiplier;
		this.#minOpaqueSpeed = minOpaqueSpeed;
		this.#particleColour = parseColour(particleColour);

		this.#texBytes = Math.ceil(Math.log2(Math.sqrt(particles)));
		const texSize = 1 << this.#texBytes;

		// console.log("particles / tex size / log2", particles, texSize, this.#texBytes);

		// Spawn *two* 2-component 32F textures to hold the positions of the
		// particles.
		// One of them shall hold the previous state, and the other the current
		// state. The GL program shall swap the in texture and the out framebuffer.
		this.#partTex1 = new glii.Texture({
			format: this.glii.gl.RG,
			internalFormat: this.glii.gl.RG32F,
			type: glii.FLOAT,
		});
		this.#partFB1 = new glii.FrameBuffer({
			color: [this.#partTex1],
			width: texSize, /// TODO: test with 256x256 = 65k particles, extend later
			height: texSize,
		});

		this.#partTex2 = new glii.Texture({
			format: this.glii.gl.RG,
			internalFormat: this.glii.gl.RG32F,
			type: glii.FLOAT,
		});
		this.#partFB2 = new glii.FrameBuffer({
			color: [this.#partTex2],
			width: texSize, /// TODO: test with 256x256 = 65k particles, extend later
			height: texSize,
		});

		// An attribute storage for drawing single particles, from *either*
		// particle position texture.
		this._particles = new this.glii.SingleAttribute({
			// Coords on the particles texture. The value of that
			// texel is the screen position of the particle.

			usage: this.glii.STATIC_DRAW,
			size: this.#particleCount,
			growFactor: 1,

			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});

		// An attribute storage for drawing two-particle segments, connecting
		// both positions from both particle position textures for each particle.
		// i.e. draw a line between each particle's previous position and last
		// position.
		this._particleSegments = new this.glii.SingleAttribute({
			// Coords on the particles texture. The value of that
			// texel is the screen position of the particle.

			// A positive (or zero) value on the X component means the
			// 0th texture; a negative value means the 1st texture.

			usage: this.glii.STATIC_DRAW,
			size: this.#particleCount * 2,
			growFactor: 1,

			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});

		// Static indices for drawing individual particles
		this._singleParticleIndices = new this.glii.SequentialIndices({
			size: this.#particleCount,
			drawMode: this.glii.POINTS,
		});

		// Static indices for drawing per-particle segments
		this._lineParticleIndices = new this.glii.SequentialIndices({
			size: this.#particleCount * 2,
			drawMode: this.glii.LINES,
		});

		// Init data for the attrib storages
		const partAttrs = new Float32Array(particles * 2);
		const partSegments = new Float32Array(particles * 2);

		for (let i = 0; i < this.#particleCount; i++) {
			// For now, have all the coordinates assume that the textures are
			// 256x256 in size.
			// let y = i >> 8;
			let y = i >> this.#texBytes;
			let x = i - (y << this.#texBytes);

			let i2 = i * 2;
			let i4 = i * 4;

			x /= texSize;
			y /= texSize;

			partAttrs[i2] = x;
			partAttrs[i2 + 1] = y;

			partSegments[i4] = x;
			partSegments[i4 + 1] = y;
			partSegments[i4] = x - 2;
			partSegments[i4 + 1] = y;
		}

		this._particles.multiSet(0, partAttrs);
		this._particleSegments.multiSet(0, partSegments);

		const randomPositions = Float32Array.from(
			new Array(texSize * texSize * 2),
			() => Math.random() * 2 - 1
		);

		// console.log(partAttrs, randomPositions);

		// Initially, load some noise into the particle position textures
		this.#partTex1.texArray(texSize, texSize, randomPositions);
		this.#partTex2.texArray(texSize, texSize, randomPositions);

		if (isFinite(particleLifetime) && particleLifetime > 0) {
			this.#respawnInterval = setInterval(
				() => this.#respawnParticles(),
				(particleLifetime * 1000) / (1 << this.#texBytes)
			);
		}
	}

	glProgramDefinition() {
		// The main program renders the particles

		const opts = super.glProgramDefinition();

		const minOpaqueSpeed = this.#minOpaqueSpeed * this.#speedMultiplier;

		return {
			...opts,
			indexBuffer: this._singleParticleIndices,
			attributes: {
				aParticleTexel: this._particles,
			},
			textures: {
				uPart: this.#partTex1,
				uField: this._fieldTexture,
			},
			vertexShaderMain: `
				vec2 particlePos = texture2D(uPart, aParticleTexel).xy;
				gl_Position = vec4(particlePos, 0., 1.);

				vec2 value = texture2D(uField, particlePos / 2.0 + 0.5).xy;

				vSpeed = length(value);

				gl_PointSize = 1.0;
			`,
			varyings: {
				// vColour: "vec4",
				vSpeed: "float",
			},
			fragmentShaderMain:
				`gl_FragColor = vec4(${glslVecNify(this.#particleColour)});` +
				`gl_FragColor.a *= vSpeed / ${glslFloatify(minOpaqueSpeed * 256)};`,
			// minOpaqueSpeed > 0
			// ? `gl_FragColor.a *= min(1., vSpeed / ${glslFloatify( minOpaqueSpeed )});`
			// : ""
		};
	}

	simulatorGlProgramDefinition() {
		// The simulator program renders to the framebuffer linked to the
		// particle position texture **not** being used as input, outputting
		// the new position vec2 for each particle.

		// The simulator program reuses the single quad defined by `Field`:
		// aPos, aUV and a sequential 4-vertex index
		const opts = super.glProgramDefinition();

		/// TODO: Apply a stencil in order to skip texels corresponding to non-existing
		/// particles

		return {
			...opts,
			// indexBuffer: this._singleParticleIndices,
			// attributes: { aParticleTexel: this._particles, },
			textures: {
				uPart: this.#partTex1,
				uField: this._fieldTexture,
			},

			uniforms: {
				uPixelSize: "vec2",
				uTimeDelta: "float",
			},

			// vertexShaderSource: `
			// void main(){
			// 	vec2 particlePos = texture2D(uPart, aParticleTexel).xy;
			// 	// vec2 value = texture2D(uField, particlePos).xy;
			//
			// 	// vNewPosition = particlePos + (value / 128.0);
			// 	vNewPosition = particlePos;
			// 	vParticleTexel = aParticleTexel;
			//
			// 	gl_Position = vec4(
			// 		aParticleTexel.x * 2.0 - 1.0,
			// 		1.0 - aParticleTexel.y * 2.0,
			// 		0.,
			// 		1.);
			// 	gl_PointSize = 1.0;
			// }
			// `,
			// varyings: {
			// 	vNewPosition: "vec2",
			// 	vParticleTexel: "vec2",
			// },

			vertexShaderSource: `
			void main(){
				gl_Position = vec4(aPos, 0., 1.);
				vUV = aUV;
			}
			`,

			varyings: { vUV: "vec2" },

			fragmentShaderSource: `
			void main(){
				vec2 particlePos = texture2D(uPart, vUV).xy;

				vec4 value = texture2D(uField, particlePos / 2.0 + 0.5);

				gl_FragColor = vec4(particlePos +
					(value.xy * uPixelSize * uTimeDelta),
				0., 1.);

			}
			`,
			target: this.#partFB2,
		};
	}

	#simulatorGlProgram;
	resize(x, y) {
		if (!this.#simulatorGlProgram) {
			this.#simulatorGlProgram = new this.glii.WebGL1Program(
				this.simulatorGlProgramDefinition()
			);

			// this._programs.addProgram(this.#simulatorGlProgram);
		}

		const dpr2 = (devicePixelRatio ?? 1) * 2;
		this._programs.setUniform("uPixelSize", [dpr2 / x, dpr2 / y]);
		this.#simulatorGlProgram.setUniform("uPixelSize", [dpr2 / x, dpr2 / y]);

		super.resize(x, y);
	}

	#lastRedrawTime;

	redraw(crs, matrix, viewportBbox) {
		// Swap texture and framebuffer
		// Also unbind the texture used in the output framebuffer; if it's
		// unused but bound to an active texture unit, it would trigger
		// illegal feedback.

		if (this.#simulatorGlProgram._target === this.#partFB2) {
			this.#partTex1.unbind();
			this.#simulatorGlProgram.setTarget(this.#partFB1);
			this.#simulatorGlProgram.setTexture("uPart", this.#partTex2);

			this._program.setTexture("uPart", this.#partTex1);
		} else {
			this.#partTex2.unbind();
			this.#simulatorGlProgram.setTarget(this.#partFB2);
			this.#simulatorGlProgram.setTexture("uPart", this.#partTex1);

			this._program.setTexture("uPart", this.#partTex2);
		}
		/// FIXME: Why is the uField texture being expelled from its unit???
		this.#simulatorGlProgram.setTexture("uField", this._fieldTexture);

		const now = performance.now();
		if (this.#lastRedrawTime) {
			this.#simulatorGlProgram.setUniform(
				"uTimeDelta",
				(this.#speedMultiplier * (now - this.#lastRedrawTime)) / 1000
			);
		}
		this.#lastRedrawTime = now;

		this.#simulatorGlProgram.run();

		// let debug1 = this.#partFB1.readPixels(0, 0, 4, 4);
		// let debug2 = this.#partFB2.readPixels(0, 0, 4, 4);
		// console.log(debug1, debug2);

		// Clear the output colour framebuffer, but not the field framebuffer.
		Acetate.prototype.clear.call(this);

		/// TODO: Extend the bounds of the field (in order to spawn particles
		/// slightly outside of the platina and simulate their entering)
		/// Take some code from `QuadBin`.

		return super.redraw(crs, matrix, viewportBbox);
	}

	// An animated Acetate is always dirty, meaning it wants to render at every
	// frame.
	get dirty() {
		return true;
	}
	set dirty(d) {
		super.dirty = d;
	}

	clear() {
		// Clear the framebuffer only if the parent functionality is dirty -
		// otherwise, keep the framebuffer dirty to draw on top and perform the fade-in.
		if (super.dirty) {
			super.clear();
		}
	}

	#respawnIndex = 0;

	// Respawns an entire texture-column worth of particles
	#respawnParticles() {
		if (!this._platina) return;

		const texSize = 1 << this.#texBytes;

		this.#respawnIndex = (this.#respawnIndex + 1) % texSize;

		const randomPositions = Float32Array.from(
			new Array(texSize * 2),
			() => Math.random() * 2 - 1
		);

		this.#partTex1.texSubArray(1, texSize, randomPositions, this.#respawnIndex, 0);
		this.#partTex2.texSubArray(1, texSize, randomPositions, this.#respawnIndex, 0);
	}
}
