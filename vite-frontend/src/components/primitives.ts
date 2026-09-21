import { tv } from "tailwind-variants";

export const title = tv({
  base: "tracking-tight inline font-semibold",
  variants: {
    color: {
      violet: "text-slate-700 dark:text-slate-200",
      yellow: "text-amber-700 dark:text-amber-300",
      blue: "text-teal-700 dark:text-teal-300",
      cyan: "text-cyan-700 dark:text-cyan-300",
      green: "text-emerald-700 dark:text-emerald-300",
      pink: "text-rose-700 dark:text-rose-300",
      foreground: "text-foreground",
    },
    size: {
      sm: "text-3xl lg:text-4xl",
      md: "text-[2.3rem] lg:text-5xl leading-9",
      lg: "text-4xl lg:text-6xl",
    },
    fullWidth: {
      true: "w-full block",
    },
  },
  defaultVariants: {
    size: "md",
  },
});
