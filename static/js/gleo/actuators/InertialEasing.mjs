/**
 *
 * Support stuff for creating an "Inertial easing".
 *
 * The core idea is that any change to the plane's center is "tweened" (short
 * for "in-betweened", from flash animators' jargon) with an ease-in-out function.
 *
 * Ease-in-out functions are not hard - the problem is aborting an easing animation
 * and starting a new one right after. This use case is very common - starting a
 * movement to a different part of the plane while a movement is already on its way,
 * or wheel-zoom interactions in very fast succesion.
 *
 * The approach to this is to have some way of calculating the inertia of the aborted
 * easing, and start a new easing with that inertia.
 *
 * The easing function **for the interpolated position** (and scale) is the one from
 * https://math.stackexchange.com/questions/121720/ease-in-out-function#121755 :
 * f(x) = x^a / ( x^a + (1-x)^a )
 * , with values of a between 0 and... 4? 5?. Typically 2 for a quadratic-like easing, or
 * 3 for a cubic-like easing.
 *
 * Assuming x and a are positive, that's equal to:
 * f(x) = 1/((1/x - 1)^a + 1)
 *
 * The **speed of change** for the interpolated position/scale is the derivative on x, namely
 *
 * f'(x) = (a (-(x - 1) x)^(a - 1))/(x^a + (1 - x)^a)^2
 *
 * The previous functions assume ranges from 0 to 1 - these have to be multiplied by the
 * delta vector (end position minus start position) to get the position at a given time.
 *
 * What if the easing started with inertia (with an initial speed)? Then, the starting speed
 * s0 will decrease polinomically - the speed at any given time will be s(x) = s0 * (1-x)^a.
 *
 * Therefore, the inertial component of the position will be the integral of that function
 * from zero to a given point in time - namely,
 * i(x,a) = s0 * (1 - (1 - x)^(1 + a))/(1 + a)
 *
 * The total amount of delta due to inertia will be:
 * s0 * (a+1)
 *
 * So the amount of easing delta needed will be the desired delta minus the inertia delta.
 *
 *
 * Special thanks to Juan Arias de Reyna <https://personal.us.es/arias/>
 * for pointers on how to approach this problem.
 */

export default class InertialEasing {
	constructor(start, end, speed, exponent = 2) {
		// `start`, `end` and `speed` are n-element arrays.
		// Typically 3-element arrays, for x-y-scale.

		/// TODO: Sanity check: `start`, `end` and `speed` should have the same number
		/// of elements

		const e = (this.exp = exponent);
		this.start = start;
		this.end = end;
		this.inertia = speed;

		const inertialDelta = speed.map((s) => s / (e + 1));
		const totalDelta = end.map((e, i) => e - start[i]);
		this.easingDelta = totalDelta.map((t, i) => t - inertialDelta[i]);
		// console.log("start, inertial, total, easing:", start, inertialDelta, totalDelta, this.easingDelta);

		// Sanity checks
		if (
			start.some(Number.isNaN) ||
			end.some(Number.isNaN) ||
			speed.some(Number.isNaN)
		) {
			throw new Error("A parameter for InertialEasing is Not A Number.");
		}
	}

	// Returns the eased values for the percentage (between 0 and 1) given
	getValues(percentage) {
		if (percentage < 0) {
			return this.start;
		}
		if (percentage > 1) {
			return this.end;
		}

		const x = percentage;
		const exp = this.exp;
		const inertiaComponent = (1 - Math.pow(1 - x, 1 + exp)) / (1 + exp);
		const easingComponent = 1 / (Math.pow(1 / x - 1, exp) + 1);

		return this.start.map(
			(s, i) =>
				s +
				this.inertia[i] * inertiaComponent +
				this.easingDelta[i] * easingComponent
		);
	}

	// Returns the speed of values change for the percentage (between 0 and 1) given
	getSpeed(percentage) {
		if (percentage < 0) {
			return;
		}
		if (percentage > 1) {
			return;
		}

		const x = percentage;
		const exp = this.exp;
		const inertiaComponent = Math.pow(1 - x, exp);
		const easingComponent =
			(exp * Math.pow(-(x - 1) * x, exp - 1)) /
			Math.pow(Math.pow(x, exp) + Math.pow(1 - x, exp), 2);

		return this.inertia.map(
			(iner, i) => iner * inertiaComponent + this.easingDelta[i] * easingComponent
		);
	}
}
