// Package domain holds Shinrin's core model: entities, value objects and
// domain errors. It is the centre of the hexagon and imports nothing from the
// rest of the module (and nothing outside the standard library).
//
// Numbers used for analysis (prices, ratios, scores) are float64. Shinrin
// analyses data, it never settles money, so binary floating point is accurate
// enough; switch a type to a decimal only if a calculation proves otherwise.
package domain
