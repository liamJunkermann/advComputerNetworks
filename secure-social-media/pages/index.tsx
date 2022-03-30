import { Grid } from "@mui/material";
import React from "react";
import { useUser } from "../config/user.context";

const Index = () => {
  const { user } = useUser();
  if (user) {
    return (
      <Grid container spacing={2}>
        Authed
      </Grid>
    );
  }
  return (
    <Grid container spacing={2}>
      unauthed
    </Grid>
  );
};

export default Index;
