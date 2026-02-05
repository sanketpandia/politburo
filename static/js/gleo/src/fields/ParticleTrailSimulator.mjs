import Acetate from "../acetates/Acetate.mjs";
import { VectorField } from "./Field.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import glslVecNify from "../util/glslVecNify.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class ParticleTrailSimulator
 * @inherits VectorField
 *
 * As `ParticleSimulator`, but displays particle trails instead of particles.
 *
 */

export default class ParticleTrailSimulator extends VectorField {
	#particleCount;
	#speedMultiplier;
	#minOpaqueSpeed;
	#particleColour;
	#fadingPercentage;
	#drawLines;
	#dotSize;

	#partTex1;
	#partFB1;
	#partTex2;
	#partFB2;

	#texBytes;

	#sections; // Or rather, section count
	#rowsPerSection;
	#sectionFraction; // rows for one section divided by total rows
	#respawnAmount;
	#msecsPerRespawnColumn;

	#sectionQuads;

	/**
	 * @constructor ParticleTrailSimulator(target: GliiFactory, opts?: ParticleTrailSimulator Options)
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
			 * @option trailSize: Number = 8
			 * Number of particle positions per trail.
			 *
			 * Must be a multiple of 2 (if not, will be rounded up).
			 *
			 */
			trailSize = 8,

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
			 * If this is set to `Infinity` or `NaN`, particles will never respawn.
			 */
			particleLifetime = 10,

			/**
			 * @option particleLifetime: Number = 1
			 * Duration of the fade-in and fade-out on particle trails, in seconds.
			 *
			 * Particles will fade-out just before respawning, and fade-in
			 * afterwards.
			 */
			fadeDuration = 1,

			/**
			 * @option drawLines: Boolean = true
			 * When `true`, the trails will be drawn as thin lines (1 device pixel wide).
			 *
			 * When `false`, the trails will be drawn as a group of squares (the
			 * size of the squares is defined via the `dotSize` option)
			 */
			drawLines = true,

			/**
			 * @option dotSize: Number = 4
			 * Size of the square dots used to draw a trail's particle positions.
			 *
			 * Whis works like the `size` option of the `Dot` symbol, and has the
			 * same limitations (size is in device pixels, not in CSS pixels;
			 * and the maximum value depends on the GPU and the WebGL/OpenGL
			 * stack).
			 *
			 * Only has effect when `drawLines` is `false`.
			 */
			dotSize = 4,

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
			particleColour = "black",

			...opts
		} = {}
	) {
		super(target, opts);
		const glii = this.glii;

		this.#particleCount = particles;
		this.#speedMultiplier = speedMultiplier;
		this.#minOpaqueSpeed = minOpaqueSpeed;
		this.#particleColour = parseColour(particleColour);
		this.#fadingPercentage = fadeDuration / particleLifetime;
		this.#drawLines = drawLines;
		this.#dotSize = dotSize;

		// How many sections per texture?
		const sections = (this.#sections = Math.ceil(trailSize / 2) + 1);

		this.#texBytes = Math.ceil(Math.log2(Math.sqrt(particles * sections)));
		const texSize = 1 << this.#texBytes;

		const rowsPerSection = (this.#rowsPerSection = Math.ceil(particles / texSize));
		this.#sectionFraction = rowsPerSection / texSize;

		// console.log(
		// 	"particles / sections / rows / fraction / tex size / log2",
		// 	particles,
		// 	sections,
		// 	rowsPerSection,
		// 	this.#sectionFraction,
		// 	texSize,
		// 	this.#texBytes
		// );

		// // Set to the last possible state, since the algorithm will advance
		// // texture offsets *before* each run.
		// this.#sectionSequence = sections - 1;

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
		// One attribute per particle-section. All the sections from a texture are
		// drawn at once.
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
		// One attribute per particle-section.

		this._particleSegments = new this.glii.SingleAttribute({
			// Coords on the particles texture. The value of that
			// texel is the screen position of the particle.

			// A value between 0 and 1 denotes the 1st texture.
			// A value between 2 and 3 denotes the 2nd texture.

			usage: this.glii.STATIC_DRAW,
			size: this.#particleCount * sections * 2,
			growFactor: 1,

			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});

		// Data for the section quads - a single section is simulated by
		// triggering a partial draw consisting of a single quad.
		this.#sectionQuads = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 4 * sections,
				growFactor: 1,
			},
			[
				{
					// Vertex position
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Texel coords
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
			]
		);

		const sectionAttrs = this.#sectionQuads.asStridedArray(0);
		// const sectionAttrs = new Float32Array(4 * sections * 4);
		for (let i = 0; i < sections; i++) {
			// Target vertical texel coordinate for this section
			const t0 = i * this.#sectionFraction;
			const t1 = (i + 1) * this.#sectionFraction;

			// Source vertical clipspace coordinate for this section
			const c0 = t0 * 2 - 1;
			const c1 = t1 * 2 - 1;

			sectionAttrs.set(
				// prettier-ignore
				[
					-1, c0, 0, t0,
					-1, c1, 0, t1,
					+1, c0, 1, t0,
					+1, c1, 1, t1,
				],
				i * 4
			);
		}
		this.#sectionQuads.commit(0, 4 * sections);

		// Static indices for drawing section quads
		this._sectionQuadIndices = new this.glii.SequentialIndices({
			size: sections * 4,
			drawMode: this.glii.TRIANGLE_STRIP,
		});

		// Static indices for drawing individual particle positions as dots
		this._singleParticleIndices = new this.glii.SequentialIndices({
			size: this.#particleCount * this.#sections,
			drawMode: this.glii.POINTS,
		});

		// Static indices for drawing per-particle segments as lines
		this._trailIndices = new this.glii.IndexBuffer({
			type: this.glii.UNSIGNED_INT,
			size: particles * sections * 4,
			drawMode: this.glii.LINES,
		});

		// Init data for the "single particles" attributes
		const partAttrs = new Float32Array(particles * sections * 2);
		// const partSegments = new Float32Array(particles * 2);

		for (let i = 0; i < this.#particleCount * this.#sections; i++) {
			// For now, have all the coordinates assume that the textures are
			// 256x256 in size.
			// let y = i >> 8;
			let y = i >> this.#texBytes;
			let x = i - (y << this.#texBytes);

			let i2 = i * 2;
			// let i4 = i * 4;

			x /= texSize;
			y /= texSize;

			partAttrs[i2] = x;
			partAttrs[i2 + 1] = y;

			// partSegments[i4] = x;
			// partSegments[i4 + 1] = y;
			// partSegments[i4] = x - 2;
			// partSegments[i4 + 1] = y;
		}

		this._particles.multiSet(0, partAttrs);
		// this._particleSegments.multiSet(0, partSegments);

		// Initial data for the "particle segments" attributes
		const segAttrs = new Float32Array(particles * sections * 4);
		for (let s = 0; s < sections; s++) {
			const secOffsetY = s * this.#sectionFraction;

			for (let t = 0; t < 2; t++) {
				const texOffsetX = t * 2;
				const sectionIdx = (s * 2 + t) * particles;

				for (let p = 0; p < particles; p++) {
					let y = p >> this.#texBytes;
					let x = p - (y << this.#texBytes);
					x /= texSize;
					x += texOffsetX;
					y /= texSize;
					y += secOffsetY;

					const i = sectionIdx + p;
					const i2 = i * 2;
					partAttrs[i2] = x;
					partAttrs[i2 + 1] = y;
				}
			}
		}
		this._particleSegments.multiSet(0, segAttrs);

		// Initial data for the line indices connecting particle-segment vertices.
		// Connect each vertex with the one (particles) slots away, wrapping when at
		// the last section.
		const trailIdxs = new Uint32Array(particles * sections * 4);

		for (let s = 0; s < this.#sections; s++) {
			const offset = s === sections - 1 ? particles * (1 - sections) : particles;
			const idx = s * particles;
			for (let p = 0; p < particles; p++) {
				const i = idx + p;
				const i2 = i * 2;
				trailIdxs[i2] = i;
				trailIdxs[i2 + 1] = i + offset;
			}
		}
		this._trailIndices.set(0, trailIdxs);

		// Initial position for the particles (only for the first section)
		const randomPositions = Float32Array.from(new Array(texSize * texSize * 2));

		randomPositions.set(
			Array.from(new Array(particles * 2), () => Math.random() * 2 - 1),
			0
		);

		this.#partTex1.texArray(texSize, texSize, randomPositions);
		// this.#partTex2.texArray(texSize, texSize, randomPositions);

		// Set up particle respawning
		if (isFinite(particleLifetime) && particleLifetime > 0) {
			this.#msecsPerRespawnColumn =
				(particleLifetime * 1000) / (1 << this.#texBytes);
			this.#respawnAmount = 1;
			if (this.#msecsPerRespawnColumn < 250) {
				this.#respawnAmount = Math.ceil(250 / this.#msecsPerRespawnColumn);
			}

			setInterval(
				() => this.#respawnParticles(),
				this.#msecsPerRespawnColumn * this.#respawnAmount
			);

			// console.log(
			// 	"lifetime / respawn interval / batch size",
			// 	particleLifetime,
			// 	this.#msecsPerRespawnColumn * this.#respawnAmount,
			// 	this.#respawnAmount
			// );
		}
	}

	glProgramDefinition() {
		// The main program renders the particles

		const opts = super.glProgramDefinition();

		const minOpaqueSpeed = this.#minOpaqueSpeed * this.#speedMultiplier;

		return {
			...opts,
			// indexBuffer: this._singleParticleIndices,
			indexBuffer: this.#drawLines
				? this._trailIndices
				: this._singleParticleIndices,
			attributes: {
				aParticleTexel: this._particles,
			},
			uniforms: {
				...opts.uniforms,
				uSectionOffset: "float",
				uRespawnColumn: "float", // between 0 and 1, compare to texel X
				uFadeWidth: "float", // How far from texel X to become opaque
			},
			textures: {
				uPart1: this.#partTex1,
				uPart2: this.#partTex2,
				uField: this._fieldTexture,
			},
			vertexShaderMain:
				`
				vec2 particlePos;
				float texelX;

				if (aParticleTexel.x < 2.0) {
					texelX = aParticleTexel.x;
					particlePos = texture2D(uPart1, aParticleTexel).xy;
				} else {
					texelX = aParticleTexel.x - 2.0;
					particlePos = texture2D(uPart2, vec2(texelX, aParticleTexel.y)).xy;
				}
				gl_Position = vec4(particlePos, 0., 1.);

				vec2 value = texture2D(uField, particlePos / 2.0 + 0.5).xy;

				vSpeed = length(value);
				` +
				(this.#drawLines
					? ""
					: `gl_PointSize = ${glslFloatify(this.#dotSize)};`) +
				(isFinite(this.#fadingPercentage) && this.#fadingPercentage > 0
					? `
				float respawnDistance = min(
					fract(texelX - uRespawnColumn),
					fract(uRespawnColumn - texelX)
				) - ${glslFloatify(this.#respawnAmount / (1 << this.#texBytes))};

				vFadeOpacity = min(1.0 , respawnDistance / uFadeWidth);
			`
					: `vFadeOpacity = 1.0;`),
			varyings: {
				// vColour: "vec4",
				vSpeed: "float",
				vFadeOpacity: "float",
			},
			fragmentShaderMain:
				`
			gl_FragColor = vec4(${glslVecNify(this.#particleColour.map((n) => n / 255))});
			gl_FragColor.a *= vFadeOpacity;
			` +
				(isFinite(minOpaqueSpeed) && minOpaqueSpeed > 0
					? `gl_FragColor.a *= min( 1.0 , vSpeed / ${glslFloatify(minOpaqueSpeed)});`
					: ""),
			// `gl_FragColor.a *= vSpeed / ${glslFloatify(minOpaqueSpeed * 256)};`,
			// minOpaqueSpeed > 0
			// ? `gl_FragColor.a *= min(1., vSpeed / ${glslFloatify( minOpaqueSpeed )});`
			// : ""
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.SRC_ALPHA,
				dstRGB: this.glii.ONE_MINUS_SRC_ALPHA,
				srcAlpha: this.glii.ONE,
				dstAlpha: this.glii.ONE_MINUS_SRC_ALPHA,
			},
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
			indexBuffer: this._sectionQuadIndices,
			attributes: {
				aPos: this.#sectionQuads.getBindableAttribute(0),
				aUV: this.#sectionQuads.getBindableAttribute(1),
			},
			textures: {
				uPart: this.#partTex1,
				uField: this._fieldTexture,
			},

			uniforms: {
				uPixelSize: "vec2",
				uTimeDelta: "float",
				uSectionOffset: "float",
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

			vertexShaderSource: /* glsl */ `
				void main() {
					gl_Position = vec4(aPos + vec2(0.0, uSectionOffset), 0.0, 1.0);
					vUV = aUV;
				}
			`,

			varyings: { vUV: "vec2" },

			fragmentShaderSource: /* glsl */ `
				void main() {
					vec2 particlePos = texture2D(uPart, vUV).xy;

					vec4 value = texture2D(uField, particlePos / 2.0 + 0.5);
					// value.x += 0.01;
					// vec2 value = vec2(0.1, 0.0);

					gl_FragColor = vec4(
						particlePos +
							// (value.xy),
							value.xy * uPixelSize * uTimeDelta,
						0.0,
						1.0
					);

				}
			`,
			target: this.#partFB1,
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

		if (isFinite(this.#fadingPercentage)) {
			this._program.setUniform("uFadeWidth", this.#fadingPercentage);
		} else {
			this._program.setUniform("uFadeWidth", 0);
		}
	}

	#lastRedrawTime = 0;
	#sectionSequence = 0;
	#respawnColumnOffset = 0;

	redraw(crs, matrix, viewportBbox) {
		// Swap texture and framebuffer
		// Also unbind the texture used in the output framebuffer; if it's
		// unused but bound to an active texture unit, it would trigger
		// illegal feedback.

		if (this.#simulatorGlProgram._target === this.#partFB1) {
			// 1→2
			this.#partTex2.unbind();
			this.#simulatorGlProgram.setTarget(this.#partFB2);
			this.#simulatorGlProgram.setTexture("uPart", this.#partTex1);

			// this._program.setTexture("uPart", this.#partTex2);

			this.#simulatorGlProgram.setUniform("uSectionOffset", 0);
		} else {
			// 1←↓2

			this.#partTex1.unbind();
			this.#simulatorGlProgram.setTarget(this.#partFB1);
			this.#simulatorGlProgram.setTexture("uPart", this.#partTex2);

			// this._program.setTexture("uPart", this.#partTex1);

			if (this.#sectionSequence >= this.#sections - 1) {
				this.#simulatorGlProgram.setUniform(
					"uSectionOffset",
					-this.#sectionFraction * (this.#sections - 1) * 2
				);
			} else {
				this.#simulatorGlProgram.setUniform(
					"uSectionOffset",
					this.#sectionFraction * 2
				);
			}
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

		const sinceLastRespawn = now - this.#lastRespawnTime;
		this.#respawnColumnOffset = Math.floor(
			sinceLastRespawn / this.#msecsPerRespawnColumn
		);

		const texSize = 1 << this.#texBytes;
		this._program?.setUniform(
			"uRespawnColumn",
			((this.#respawnIndex + this.#respawnColumnOffset) % texSize) / (texSize - 1)
		);

		this.#lastRedrawTime = now;

		this.#simulatorGlProgram.runPartial(4 * this.#sectionSequence, 4);

		// let debug1 = this.#partFB1.readPixels(0, 0, 4, 4);
		// let debug2 = this.#partFB2.readPixels(0, 0, 4, 4);
		//
		// let debug1fmt = Array.from(new Array(debug1.length / 4), (_,i)=>[debug1[i*4], debug1[i*4+1]]);
		// let debug2fmt = Array.from(new Array(debug1.length / 4), (_,i)=>[debug2[i*4], debug2[i*4+1]]);
		//
		// // console.log(debug1, debug2);
		// console.log(debug1fmt , debug2fmt);

		if (this.#simulatorGlProgram._target === this.#partFB1) {
			this.#sectionSequence++;
			this.#sectionSequence %= this.#sections;
		}

		// Clear the output colour framebuffer, but not the field framebuffer.
		Acetate.prototype.clear.call(this);

		/// TODO: Extend the bounds of the field (in order to spawn particles
		/// slightly outside of the platina and simulate their entering)
		/// Take some code from `QuadBin`.

		return super.redraw(crs, matrix, viewportBbox);
	}

	runProgram() {
		if (!this.#drawLines) {
			return super.runProgram();
		}

		// On a given draw call, draw only the lines between connected sections.
		// In other words: skip the connection between the "tail" and the "head"
		// of each trail.
		// This is achieved with (up to) two `drawPartial` calls, knowing the
		// structure of this._trailIndices. This skips the section connecting the
		// tail and head.

		if (this.#sectionSequence !== 0) {
			this._program.runPartial(0, this.#sectionSequence * this.#particleCount * 2);
		}

		const s = this.#sections - this.#sectionSequence - 1;
		if (s !== 0) {
			this._programs.runPartial(
				(this.#sectionSequence + 1) * this.#particleCount * 2,
				s * this.#particleCount * 2
			);
		}
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
	#lastRespawnTime = 0;

	// Respawns an entire texture-column worth of particles
	#respawnParticles() {
		if (!this._platina) return;

		const texSize = 1 << this.#texBytes;

		let w = Math.min(
			Math.max(this.#respawnColumnOffset, this.#respawnAmount),
			texSize - this.#respawnIndex
		);
		// console.log("respawning at column ", this.#respawnIndex,w, this.#respawnIndex+w, texSize);

		const sectionRandomPositions = Array.from(
			new Array(w * this.#rowsPerSection * 2),
			() => Math.random() * 2 - 1
		);

		const columnRandomPositions = Float32Array.from(
			new Array(this.#sections).fill(sectionRandomPositions).flat()
		);

		const h = this.#sections * this.#rowsPerSection;

		// if (this.#simulatorGlProgram._target === this.#partFB1) {
		this.#partTex1.texSubArray(w, h, columnRandomPositions, this.#respawnIndex, 0);
		// } else {
		this.#partTex2.texSubArray(w, h, columnRandomPositions, this.#respawnIndex, 0);
		// }

		this.#respawnIndex = (this.#respawnIndex + w) % texSize;

		this._program?.setUniform("uRespawnColumn", this.#respawnIndex / (texSize - 1));

		this.#lastRespawnTime = performance.now();
	}
}
