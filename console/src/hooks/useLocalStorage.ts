import { useCallback, useState } from "react";

interface StoredValue<T> {
  version: number;
  value: T;
}

function readValue<T>(key: string, initialValue: T, version: number): T {
  try {
    const item = window.localStorage.getItem(key);
    if (item === null) return initialValue;

    const parsed: StoredValue<T> = JSON.parse(item);
    if (parsed.version !== version) return initialValue;

    return parsed.value;
  } catch {
    return initialValue;
  }
}

function saveValue<T>(key: string, value: T, version: number): void {
  try {
    const toStore: StoredValue<T> = { version, value };
    window.localStorage.setItem(key, JSON.stringify(toStore));
  } catch {
    // Silently fail on quota exceeded or other storage errors
  }
}

export default function useLocalStorage<T>(
  key: string,
  initialValue: T,
  version = 1,
): [T, (value: T | ((prev: T) => T)) => void] {
  const [storedValue, setStoredValue] = useState<T>(() =>
    readValue(key, initialValue, version),
  );

  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      setStoredValue((prev) => {
        const newValue = value instanceof Function ? value(prev) : value;
        saveValue(key, newValue, version);
        return newValue;
      });
    },
    [key, version],
  );

  return [storedValue, setValue];
}