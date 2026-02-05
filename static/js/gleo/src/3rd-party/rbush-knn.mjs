/**
 * ISC License.
 *
 * Copyright (c) 2016, Vladimir Agafonkin
 *
 * Permission to use, copy, modify, and/or distribute this software for any purpose
 * with or without fee is hereby granted, provided that the above copyright notice
 * and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
 * REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
 * FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
 * INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
 * OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
 * TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
 * THIS SOFTWARE.
 */

import Queue from './tinyqueue.mjs';

export default function knn(tree, x, y, n, predicate, maxDistance) {
    var node = tree.data,
        result = [],
        toBBox = tree.toBBox,
        i, child, dist, candidate;

    var queue = new Queue(undefined, compareDist);

    while (node) {
        for (i = 0; i < node.children.length; i++) {
            child = node.children[i];
            dist = boxDist(x, y, node.leaf ? toBBox(child) : child);
            if (!maxDistance || dist <= maxDistance * maxDistance) {
                queue.push({
                    node: child,
                    isItem: node.leaf,
                    dist: dist
                });
            }
        }

        while (queue.length && queue.peek().isItem) {
            candidate = queue.pop().node;
            if (!predicate || predicate(candidate))
                result.push(candidate);
            if (n && result.length === n) return result;
        }

        node = queue.pop();
        if (node) node = node.node;
    }

    return result;
}

function compareDist(a, b) {
    return a.dist - b.dist;
}

function boxDist(x, y, box) {
    var dx = axisDist(x, box.minX, box.maxX),
        dy = axisDist(y, box.minY, box.maxY);
    return dx * dx + dy * dy;
}

function axisDist(k, min, max) {
    return k < min ? min - k : k <= max ? 0 : k - max;
}
