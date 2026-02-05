// From https://stackoverflow.com/questions/15095909/from-rgb-to-hsv-in-opengl-glsl/17897228#17897228 :

const hsv2rgb = `
const vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);

vec3 hsv2rgb(vec3 c)
{
    vec3 p = abs(fract(c.xxx + K.xyz) * 6.0 - K.www);
    return c.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), c.y);
}
`;

export default hsv2rgb;
