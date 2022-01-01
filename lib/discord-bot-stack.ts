import * as cdk from "@aws-cdk/core";
import * as lambda from "@aws-cdk/aws-lambda";
import * as apigatewayv2 from "@aws-cdk/aws-apigatewayv2";
import { HttpLambdaIntegration } from "@aws-cdk/aws-apigatewayv2-integrations";
import * as ssm from "@aws-cdk/aws-ssm";
import * as kms from "@aws-cdk/aws-kms";
import * as iam from "@aws-cdk/aws-iam";
import * as path from "path";
import { spawnSync, SpawnSyncOptions } from "child_process";

export class APIStack extends cdk.Stack {
  constructor(scope: cdk.Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const ssmpath = process.env.SSMPATH!;

    // 実行時環境変数
    const commandsEnv = {
      TIMEZONE: "Asia/Tokyo",
      SSMPATH: ssmpath,
    };

    // go のコードの場所
    const handlerEntry = path.join(__dirname, "../functions/discordbot");
    const commandsEntry = path.join(__dirname, "../functions/commands");
    // go build 用の環境変数
    const buildEnv = {
      CGO_ENABLED: "0",
      GOOS: "linux",
      GOARCH: "amd64",
    };

    // kms ssm key
    const kmskey = kms.Key.fromLookup(this, `KMSKey`, {
      aliasName: "alias/aws/ssm",
    });
    const commands = new lambda.Function(this, `Commands`, {
      // 実行時の環境変数を定義
      environment: commandsEnv,
      code: lambda.Code.fromAsset(commandsEntry, {
        bundling: {
          // try to bundle on the local machine
          local: {
            tryBundle(outputDir: string) {
              // make sure that we have all the required
              // dependencies to build the executable locally.
              // In this case we just check to make sure we have
              // go installed
              try {
                exec("go version", {
                  stdio: [
                    // show output
                    "ignore", //ignore stdio
                    process.stderr, // redirect stdout to stderr
                    "inherit", // inherit stderr
                  ],
                });
              } catch {
                // if we don't have go installed return false which
                // tells the CDK to try Docker bundling
                return false;
              }

              exec(
                [
                  "go get -v ./...", // get package
                  "go test -v", // run tests first
                  `go build -o ${path.join(outputDir, "bootstrap")}`,
                ].join(" && "),
                {
                  env: { ...process.env, ...buildEnv }, // environment variables to use when running the build command
                  stdio: [
                    // show output
                    "ignore", //ignore stdio
                    process.stderr, // redirect stdout to stderr
                    "inherit", // inherit stderr
                  ],
                  cwd: commandsEntry, // where to run the build command from
                }
              );
              return true;
            },
          },
          image: lambda.Runtime.GO_1_X.bundlingImage, // lambci/lambda:build-go1.x
          command: [
            "bash",
            "-c",
            [
              "go get -v ./...",
              "go test -v",
              "go build -o /asset-output/bootstrap",
            ].join(" && "),
          ],
          environment: buildEnv,
        },
      }),
      handler: "bootstrap", // if we name our handler 'bootstrap' we can also use the 'provided' runtime
      runtime: lambda.Runtime.GO_1_X,
    });

    // 環境変数
    const handlerEnv = {
      TIMEZONE: "Asia/Tokyo",
      SSMPATH: ssmpath,
      PUBKEY: process.env.DISCORD_APP_PUBLIC_KEY!,
      CMDFUNC: commands.functionArn,
    };
    // lambdaの定義
    const handler = new lambda.Function(this, `Handler`, {
      // 実行時の環境変数を定義
      environment: handlerEnv,
      code: lambda.Code.fromAsset(handlerEntry, {
        bundling: {
          // try to bundle on the local machine
          local: {
            tryBundle(outputDir: string) {
              // make sure that we have all the required
              // dependencies to build the executable locally.
              // In this case we just check to make sure we have
              // go installed
              try {
                exec("go version", {
                  stdio: [
                    // show output
                    "ignore", //ignore stdio
                    process.stderr, // redirect stdout to stderr
                    "inherit", // inherit stderr
                  ],
                });
              } catch {
                // if we don't have go installed return false which
                // tells the CDK to try Docker bundling
                return false;
              }

              exec(
                [
                  "go get -v ./...", // get package
                  "go test -v", // run tests first
                  `go build -o ${path.join(outputDir, "bootstrap")}`,
                ].join(" && "),
                {
                  env: { ...process.env, ...buildEnv }, // environment variables to use when running the build command
                  stdio: [
                    // show output
                    "ignore", //ignore stdio
                    process.stderr, // redirect stdout to stderr
                    "inherit", // inherit stderr
                  ],
                  cwd: handlerEntry, // where to run the build command from
                }
              );
              return true;
            },
          },
          image: lambda.Runtime.GO_1_X.bundlingImage, // lambci/lambda:build-go1.x
          command: [
            "bash",
            "-c",
            [
              "go get -v ./...",
              "go test -v",
              "go build -o /asset-output/bootstrap",
            ].join(" && "),
          ],
          environment: buildEnv,
        },
      }),
      handler: "bootstrap", // if we name our handler 'bootstrap' we can also use the 'provided' runtime
      runtime: lambda.Runtime.GO_1_X,
    });

    // iam roleの作成と関数への紐付け
    kmskey.grantDecrypt(handler);
    kmskey.grantDecrypt(commands);
    const ssmpathast = path.join(ssmpath,"*")
    const policy = new iam.Policy(this, `Policy`, {
      statements: [
        new iam.PolicyStatement({
          effect: iam.Effect.ALLOW,
          actions: ["ssm:GetParameter"],
          resources: [
            `arn:aws:ssm:${this.region}:${this.account}:parameter${ssmpathast}`,
          ],
        }),
      ],
    })
    handler.role?.attachInlinePolicy(policy);
    commands.role?.attachInlinePolicy(policy);
    commands.grantInvoke(handler);

    const httpApi = new apigatewayv2.HttpApi(this, "HttpApi");

    httpApi.addRoutes({
      path: "/endpoint",
      methods: [apigatewayv2.HttpMethod.GET, apigatewayv2.HttpMethod.POST],
      integration: new HttpLambdaIntegration("handler", handler),
    });
    /*
        const httpApi = new apigatewayv2.HttpApi(this, 'endpoint', {
          createDefaultStage: true,
          corsPreflight: {
            allowMethods: [ 
                apigatewayv2.CorsHttpMethod.GET,
                apigatewayv2.CorsHttpMethod.POST,
            ],
            allowOrigins: ['*']
          }
        });

        httpApi.addRoutes({
          path: '/endpoint',
          methods: [ apigatewayv2.HttpMethod.GET, apigatewayv2.HttpMethod.POST ],
          integration: new apigatewayv2.LambdaProxyIntegration({
            handler
          }),
        });
        */

    new cdk.CfnOutput(this, "ApiUrlOutput", { value: httpApi.url! });
  }
}

function exec(command: string, options?: SpawnSyncOptions) {
  const proc = spawnSync("bash", ["-c", command], options);

  if (proc.error) {
    throw proc.error;
  }

  if (proc.status != 0) {
    if (proc.stdout || proc.stderr) {
      throw new Error(
        `[Status ${proc.status}] stdout: ${proc.stdout
          ?.toString()
          .trim()}\n\n\nstderr: ${proc.stderr?.toString().trim()}`
      );
    }
    throw new Error(`go exited with status ${proc.status}`);
  }

  return proc;
}
