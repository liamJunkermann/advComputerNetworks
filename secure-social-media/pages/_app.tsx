import {
  Box,
  Container,
  createTheme,
  CssBaseline,
  ThemeProvider,
} from "@mui/material";
import type { AppProps } from "next/app";
import Head from "next/head";
import Header from "../components/Header";
import { UserContext } from "../config/user.context";
import { useUserData } from "../config/hooks";
import "../styles/globals.css";

function App({ Component, pageProps }: AppProps) {
  const mdTheme = createTheme();
  const userData = useUserData();
  return (
    <>
      <Head>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <UserContext.Provider value={userData}>
        <ThemeProvider theme={mdTheme}>
          <CssBaseline />
          <Box
            component="main"
            sx={{
              backgroundColor: (theme) =>
                theme.palette.mode === "light"
                  ? theme.palette.grey[100]
                  : theme.palette.grey[900],
              flexGrow: 1,
              height: "100vh",
              overflow: "auto",
            }}
          >
            <Header />
            <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
              <Component {...pageProps} />
            </Container>
          </Box>
        </ThemeProvider>
      </UserContext.Provider>
    </>
  );
}

export default App;
