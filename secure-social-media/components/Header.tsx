import React from "react";
import Link from "next/link";
import { AppBar, Box, Button, Toolbar, Typography } from "@mui/material";
import Image from "next/image";
import { useUser } from "../config/user.context";
import { auth } from "../config/firebase";

export default function Header() {
  const { user } = useUser();

  return (
    <Box sx={{ flexGrow: 1 }}>
      <AppBar position="static">
        <Toolbar>
          <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
            Encrypted Social Media
          </Typography>
          {user && (
            <>
              <Button color="inherit" onClick={() => auth.signOut()}>
                Logout
              </Button>
            </>
          )}

          {!user && (
            <Link href="/auth" passHref>
              <Button color="inherit">Login</Button>
            </Link>
          )}
        </Toolbar>
      </AppBar>
    </Box>
  );
}
