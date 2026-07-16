export function boundedAdd(a: number, b: number, maximum: number): number {
  const sum = a + b;
  return sum > maximum ? maximum : sum;
}
