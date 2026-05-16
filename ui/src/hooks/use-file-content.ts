import { useEffect, useState } from "react";
import fetch from "@/utils/fetch-throw";

export default function useFileContent(url: string) {
  const [response, setResponse] = useState("");
  const [validating, setValidating] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch(url)
      .then((res) => res.text())
      .then(setResponse)
      .catch((error) => setError(error.message))
      .finally(() => setValidating(false));
  }, [url]);
  return { error, response, validating };
}
