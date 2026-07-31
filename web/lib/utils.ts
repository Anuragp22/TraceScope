import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Merge conditional class names, letting later Tailwind utilities win over
 * earlier ones that target the same property. Every shadcn component imports
 * this, so it has to exist before any of them will compile.
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
