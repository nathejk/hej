// Client-side image compression before upload (PRD 019 §8, task 317).
//
// # Why the client compresses at all, when the server already does
//
// The BFF re-encodes everything it stores (task 303), so this is not about what ends up on disk. It
// is about what travels: a modern phone photograph is 3–8 MB, a glimt carries up to ten of them, and
// the upload happens from a field at night on one bar of signal. Sending originals would mean a
// member watching a progress bar for several minutes and a post that fails halfway more often than
// it succeeds.
//
// The server still enforces its own limits and does its own re-encode, because a client-side
// reduction is a courtesy and never a control.
//
// # The decision is separated from the canvas work
//
// `targetSize()` is pure and tested; `compressImage()` needs a DOM and is not. That split is the
// skill's rule for a `node` test environment: extract the decision, leave the plumbing.

/** The longest edge we upload. Above the server's 1600px display size, so the server still decides. */
export const UPLOAD_MAX_EDGE = 2048

/** JPEG quality for the re-encode. */
export const UPLOAD_QUALITY = 0.82

/**
 * The dimensions to scale an image to.
 *
 * **Never upscales.** A small photograph stays small rather than being blown up into a larger file
 * with no more detail in it — which would make this function actively harmful for exactly the
 * uploads that are already cheap.
 *
 * Preserves the aspect ratio, and rounds so no dimension lands on zero: a 4000×1 panorama scaled
 * naively would round its height to 0 and produce a canvas that cannot be drawn.
 */
export function targetSize(
  width: number,
  height: number,
  maxEdge: number = UPLOAD_MAX_EDGE,
): { width: number; height: number } {
  if (width <= 0 || height <= 0) return { width: 0, height: 0 }
  const longest = Math.max(width, height)
  if (longest <= maxEdge) return { width, height }

  const scale = maxEdge / longest
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

/**
 * Whether a file is worth compressing.
 *
 * Videos are passed through untouched: re-encoding one in a canvas is not possible, and task 322
 * owns what happens to them. A file already under the threshold is passed through too — spending a
 * decode and a re-encode to save nothing costs a phone battery and can make the file *bigger*, since
 * a re-encoded JPEG of an already-compressed JPEG is not reliably smaller.
 */
export function shouldCompress(type: string, bytes: number, threshold = 512 * 1024): boolean {
  return type.startsWith('image/') && bytes > threshold
}

/**
 * Downscale and re-encode an image file, returning a Blob.
 *
 * Returns the original file when anything at all goes wrong — an unsupported format, a decode
 * failure, a canvas the browser refuses to allocate for a very large image. The server can handle
 * the original, so falling back costs bandwidth rather than the post. Failing here would lose a
 * photograph over an optimisation.
 *
 * Uses `createImageBitmap` where available: it decodes off the main thread, which on a mid-range
 * Android phone is the difference between a composer that stutters while you add ten photos and one
 * that does not.
 */
export async function compressImage(file: File): Promise<Blob> {
  if (!shouldCompress(file.type, file.size)) return file
  if (typeof document === 'undefined') return file

  try {
    const bitmap = await decode(file)
    const { width, height } = targetSize(bitmap.width, bitmap.height)
    if (width === 0 || height === 0) return file

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    ctx.drawImage(bitmap, 0, 0, width, height)
    if ('close' in bitmap) bitmap.close()

    const blob = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, 'image/jpeg', UPLOAD_QUALITY),
    )
    // Only take the re-encode if it actually helped. A re-encoded JPEG is not reliably smaller than
    // its source, and uploading a *larger* file than the member chose would be the opposite of the
    // point.
    if (!blob || blob.size >= file.size) return file
    return blob
  } catch {
    return file
  }
}

async function decode(file: File): Promise<ImageBitmap | HTMLImageElement> {
  if (typeof createImageBitmap === 'function') {
    return await createImageBitmap(file)
  }
  const url = URL.createObjectURL(file)
  try {
    return await new Promise<HTMLImageElement>((resolve, reject) => {
      const img = new Image()
      img.onload = () => resolve(img)
      img.onerror = () => reject(new Error('decode failed'))
      img.src = url
    })
  } finally {
    URL.revokeObjectURL(url)
  }
}
