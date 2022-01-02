import * as cdk from "@aws-cdk/core";
import * as lambda from "@aws-cdk/aws-lambda";
import * as apigatewayv2 from "@aws-cdk/aws-apigatewayv2";
import { HttpLambdaIntegration } from "@aws-cdk/aws-apigatewayv2-integrations";
import * as ssm from "@aws-cdk/aws-ssm";
import * as kms from "@aws-cdk/aws-kms";
import * as iam from "@aws-cdk/aws-iam";
import * as dynamodb from "@aws-cdk/aws-dynamodb";
import * as sqs from "@aws-cdk/aws-sqs";
import * as path from "path";
import { spawnSync, SpawnSyncOptions } from "child_process";

export class APIStack extends cdk.Stack {
    constructor(scope: cdk.Construct, id: string, props?: cdk.StackProps) {
        super(scope, id, props);

        const ssmpath = process.env.SSMPATH!;


        // go のコードの場所
        const handlerEntry = path.join(__dirname, "../functions/discordbot");
        const commandsEntry = path.join(__dirname, "../functions/commands");
        // go build 用の環境変数
        const buildEnv = {
            CGO_ENABLED: "0",
            GOOS: "linux",
            GOARCH: "amd64",
        };
        const table = new dynamodb.Table(this, 'Table', {
            partitionKey: { name: 'id', type: dynamodb.AttributeType.STRING },
            removalPolicy: cdk.RemovalPolicy.DESTROY,
            readCapacity: 1,
            writeCapacity: 1,
        });
        const queue = new sqs.Queue(this, 'Queue');

        // 実行時環境変数
        const commandsEnv = {
            TIMEZONE: "Asia/Tokyo",
            SSMPATH: ssmpath,
            QUEUEURL: queue.queueUrl,
            TABLENAME: table.tableName,
        };

        // kms ssm key
        const kmskey = kms.Key.fromLookup(this, `KMSKey`, {
            aliasName: "alias/aws/ssm",
        });
        const handlerName = "bootstrap"
        const commands = new lambda.Function(this, `Commands`, {
            environment: commandsEnv,
            code: code(commandsEntry, handlerName, buildEnv),
            handler: handlerName, // if we name our handler 'bootstrap' we can also use the 'provided' runtime
            runtime: lambda.Runtime.GO_1_X,
        });


        // 環境変数
        const handlerEnv = {
            TIMEZONE: "Asia/Tokyo",
            SSMPATH: ssmpath,
            PUBKEY: process.env.DISCORD_APP_PUBLIC_KEY!,
            CMDFUNC: commands.functionArn,
            QUEUEURL: queue.queueUrl,
            TABLENAME: table.tableName,
        };
        // lambdaの定義
        const handler = new lambda.Function(this, `Handler`, {
            environment: handlerEnv,
            code: code(handlerEntry, handlerName, buildEnv),
            handler: handlerName, // if we name our handler 'bootstrap' we can also use the 'provided' runtime
            runtime: lambda.Runtime.GO_1_X,
        });

        // iam roleの作成と関数への紐付け
        table.grantFullAccess(handler)
        table.grantFullAccess(commands)
        queue.grantSendMessages(handler)
        queue.grantSendMessages(commands)
        kmskey.grantDecrypt(handler)
        kmskey.grantDecrypt(commands)
        const ssmpathast = path.join(ssmpath, "*")
        const policy = new iam.Policy(this, `Policy`, {
            statements: [
                new iam.PolicyStatement({
                    effect: iam.Effect.ALLOW,
                    actions: ["ssm:GetParameter"],
                    resources: [
                        `arn:aws:ssm:${this.region}:${this.account}:parameter${ssmpathast}`,
                    ],
                }),
                new iam.PolicyStatement({
                    effect: iam.Effect.ALLOW,
                    actions: ["ec2:StartInstances"],
                    resources: ["*" ],
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

function code(codePath: string, handlerName: string, buildEnvs: { [key: string]: string }): lambda.AssetCode {
    return lambda.Code.fromAsset(codePath, {
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
                            `go build -o ${path.join(outputDir, handlerName)}`,
                        ].join(" && "),
                        {
                            env: { ...process.env, ...buildEnvs }, // environment variables to use when running the build command
                            stdio: [
                                // show output
                                "ignore", //ignore stdio
                                process.stderr, // redirect stdout to stderr
                                "inherit", // inherit stderr
                            ],
                            cwd: codePath, // where to run the build command from
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
                    `go build -o /asset-output/${handlerName}`,
                ].join(" && "),
            ],
            environment: buildEnvs,
        },
    })
}
