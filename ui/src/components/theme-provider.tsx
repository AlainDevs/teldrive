import { createContext, useContext, useEffect, useMemo } from "react";
import { useLocalStorage } from "usehooks-ts";

type Theme = "dark" | "light" | "system";

interface ColorScheme {
  color: string;
  cssVars?: Record<string, string>;
}

interface ThemeProviderProps {
  children: React.ReactNode;
  defaultTheme?: Theme;
  storageKey?: string;
}

interface ThemeProviderState {
  colorScheme: ColorScheme;
  theme: Theme;
  setTheme: (theme: Theme) => void;
  setColorScheme: (colorScheme: ColorScheme) => void;
}

export const defaultColorScheme: ColorScheme = {
  color: "#82b1ff",
};

const initialState: ThemeProviderState = {
  colorScheme: defaultColorScheme,
  setColorScheme: () => null,
  setTheme: () => null,
  theme: "system",
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

export function ThemeProvider({
  children,
  defaultTheme = "system",
  storageKey = "theme",
  ...props
}: ThemeProviderProps) {
  const [colorScheme, setColorScheme] = useLocalStorage<ColorScheme>(
    "colorScheme",
    defaultColorScheme,
  );

  const [theme, setTheme] = useLocalStorage<Theme>(storageKey, defaultTheme);

  useEffect(() => {
    const root = window.document.documentElement;
    root.classList.remove("light", "dark");

    if (theme === "system") {
      const systemTheme = window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
      root.classList.add(systemTheme);
      return;
    }

    root.classList.add(theme);
  }, [theme]);

  const value = useMemo(
    () => ({
      colorScheme,
      setColorScheme,
      setTheme,
      theme,
    }),
    [theme, setTheme, colorScheme, setColorScheme],
  );

  return (
    <ThemeProviderContext.Provider {...props} value={value}>
      {children}
    </ThemeProviderContext.Provider>
  );
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext);

  if (context === undefined) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }

  return context;
};
