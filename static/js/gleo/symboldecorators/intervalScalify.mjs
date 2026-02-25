
import intervalify from "./intervalify.mjs";



export default function intervalScalify(base) {

	const intervalified = intervalify(base);

	class intervalScalifiedAcetate extends intervalified.Acetate {

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			if (!opts.attributes.aExtrude) {
				throw new Error("intervalScalify can only be applied to extruded point symbols");
			}

			const vertexMain = opts.vertexShaderMain
				.replace(`aExtrude`, `intervalExtrude`)
				.replace(
				`vIntervalOpacity = mix(uIntervalOpacity.x, uIntervalOpacity.y, intervalMidPosition);`,

				`
				intervalExtrude *= 2.0 * intervalMidPosition;

				vIntervalOpacity = mix(uIntervalOpacity.x, uIntervalOpacity.y, intervalMidPosition);`
			);

			return {
				...opts,
				vertexShaderMain: `
				vec2 intervalExtrude = aExtrude;
				` + vertexMain,
			};
		}
	}


	return class intervalScalifiedSymbol extends intervalified {
		static Acetate = intervalScalifiedAcetate;
	}

}
