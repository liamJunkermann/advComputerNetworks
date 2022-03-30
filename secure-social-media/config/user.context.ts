import { User } from "firebase/auth";
import React, { createContext, useContext } from "react";

export const UserContext = createContext<{
  user: User | undefined | null;
}>({ user: null });
UserContext.displayName = "UserContext";
export const useUser = () => useContext(UserContext);
