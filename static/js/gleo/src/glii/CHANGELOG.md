# master

# 0.11.1
- Fixed typo in `drawMePartial`
- Fixed resizing in OSX+Safari

# 0.11.0
- Implemented `Texture.texSubArray()`
- Calling `Texture.unbind()` actually unbinds the texture (binds `undefined` to its unit)

# 0.10.0
- Include CSS/device pixel size in `resize` event

# 0.9.0

- Degugging facilities: debug dumps for interleaved attribute storage, textures, framebuffer attachments
- Implemented `MultiProgram.removeProgram()`/`replaceProgram()`
- Implemented (static) `Texture.flushTextureUnits()`
- Completed `IndexBuffer.destroy()`

# 0.8.1

- Bugfix headless resizing

# 0.8.0

- `GliiFactory` includes a `ResizeObserver`, for hi-DPI screen scenarios.

# 0.7.2

- Bugfix `debugDumpAttributes`
- Allow R32F textures to be debug-dumped to a canvas
- Constants for WebGL2 `Renderbuffer` formats

# 0.7.1

- Perf: _drawingBufferSizeChanged
- Bugfix `LoDAllocator` deallocation

# 0.7.0 (2023-07-25)

- New `IndexBuffer` method: `truncate()`
- Modified method signature of `LoDIndices.forEachBlock()` and `LoDAllocator.forEachBlock()` for compatibility
- Removed diagnostic code regarding absolute texture count in `Texture`
- Bugfixed framebuffer options
- Bugfixed `SingleAttribute.setNumber()`
- New blend mode constants `MIN` and `MAX`

# 0.6.0 (2023-03-14)

- `WebGL1Program` now accepts `bool`/`bvec2`/`bvec3`/`bvec4` uniforms
- New `WebGL1Program` method: `getUniform()`

# 0.5.0 (2023-03-08)

- New option for `WebGL1Program`: `unusedWarning`

# 0.4.5 (2022-01-20)

- Bugfix TriangleIndices grow during allocation
- New method: `LoDIndices.copyWithin()`

# 0.4.4 (2022-12-29)

- Do not (try to) generate `Texture`'s mipmaps when not needed

# 0.4.3 (2022-12-22)

- Work around a bug in `headless-gl`: https://github.com/stackgl/headless-gl/issues/244

# 0.4.2 (2022-12-16)

- Allocating slots in a `SparseIndices` also grows the buffer

# 0.4.1 (2022-12-14)

- Debug grow+commit logic for `IndexBuffer`

# 0.4.0 (2022-12-14)

- New set of methods to batch update (growable) `IndexBuffer`s
  - New method: `IndexBuffer.asTypedArray()`
  - New method: `IndexBuffer.commit()`
- Commented out some `getParameter()` calls, hopefully improves performance
- Fix behaviour of `AbstractAttributeSet.commit()`

# 0.3.0 (2022-12-12)

- New set of methods to batch-update (growable) attribute sets
  - New class: `StridedTypedArray` (one for each kind of `TypedArray`)
  - New method: `SingleAttribute.asStridedArray()`
  - New method: `InterleavedAttributes.asStridedArray()`
  - New method: `AbstractAttributeSet.commit()`

# 0.2.0 (2022-12-03)

- New method: `WebGL1Program.debugDumpAttributes()`
- New method: `BindableAttribute.debugDump()`
- New class: `SequentialSparseIndices`

# 0.1.1 (2022-11-14)

- Armor against errors when adding zero-sized datasets
- Prettify `VerboseAllocator` messages

# 0.1.0 (2022-09-09)

-   Multiprograms can be destroyed
-   Fixed edge case in deallocating from `LoDAllocator`
-   Framebuffers can now read back pixels from `R32F` textures
-   Implemented blend modes on a per-program basis

# 0.0.0-alpha16 (2022-08-03)

-   Revert premature optimization on texture unit allocation

# 0.0.0-alpha15 (2022-07-28)

-   New class: `VerboseAllocator` (for debugging purposes)
-   Added constants and checks for floating-point texture types

# 0.0.0-alpha14 (2022-04-20)

-   Resizable framebuffers

# 0.0.0-alpha13 (2022-01-19)

-   Bugfixed `PointIndices`' draw mode not propagating to parent class

# 0.0.0-alpha12 (2022-01-07)

-   Bugfixed `MultiProgram.run()` behaviour when a child program uses a `LoDIndices`

# 0.0.0-alpha11 (2022-01-05)

-   Implemented `LoDIndices` (and supporting `LoDAllocator`)

# 0.0.0-alpha10 (2021-12-25)

-   Implemented `MultiProgram`

# 0.0.0-alpha9 (2021-12-10)

-   `PointIndices` is now in the default exports.
-   GLSL types of attributes, varyings, uniforms now accept a precision qualifier (#20)
-   New method: `WebGL1Program.setTexture()`
-   New method: `WebGL1Program.runPartial()`
-   New method: `FrameBuffer.readPixels()`
-   New method: `Texture.asImageData()`
-   New method: `Texture.debugIntoCanvas()` (#26)

# 0.0.0-alpha8 (2021-04-13)

-   Implemented `multiSet()` methods on attribute buffers (both `Single` and `Interleaved`).
-   Running `npm run test` now runs tests on four platforms:
    -   Headless stackgl
    -   Chromium + swiftshader (software rendering)
    -   Chromium + EGL (hardware rendering)
    -   Firefox
-   Some improvements to the Leafdoc documentation

# 0.0.0-alpha7 (2021-04-09)

-   Implemented `destroy()` methods.

# 0.0.0-alpha6 (2021-02-24)

-   Implemented growable `IndexBuffer`s, plus tests.

# 0.0.0-alpha5 (2021-02-17)

-   Added `Quad` class
-   Added `WireFrameTriangleIndices` class

# 0.0.0-alpha4 (2021-01-06)

-   First public release
