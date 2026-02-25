/**
 * @class Evented
 * @inherits EventTarget
 *
 * Lightweight utility wrapper around `EventTarget`.
 */

export default class Evented extends EventTarget {
	/**
	 * @section Event Methods
	 * @method on(eventName: String, handler: Function): this
	 * Alias to `EventTarget`'s `addEventListener`.
	 * @method off(eventName: String, handler: Function): this
	 * Alias to `EventTarget`'s `removeEventListener`.
	 */
	on() {
		this.addEventListener.apply(this, arguments);
		return this;
	}
	off() {
		this.removeEventListener.apply(this, arguments);
		return this;
	}

	/**
	 * @method once(eventName: String, handler?: Function): Promise
	 * As `on()`, but the handler function will only be called once (it'll be
	 * detached after the first fired event). Returns a `Promise` that resolves
	 * to the event when that event is fired.
	 */
	once(eventName, handler) {
		return new Promise((resolve) => {
			if (handler) {
				this.on(eventName, handler, { once: true });
			}
			this.on(eventName, resolve, { once: true });
		});
	}

	/**
	 * @method fire(eventName: String, detail: Object): Boolean
	 *
	 * Wrapper over `EventTarget`'s `dispatchEvent`. Creates a new instance of
	 * `CustomEvent`, dispatches it, and returns `true` if some event handler
	 * did `preventDefault` the event.
	 */
	fire(eventName, detail) {
		return this.dispatchEvent(new CustomEvent(eventName, { detail }));
	}
}
