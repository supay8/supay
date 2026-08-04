import axios from 'axios'
import * as xml2js from 'xml2js'

export interface SoapRequestOptions {
    url: string;
    action: string;
    xmlBody: string;
}

export class SoapClient {
    static async sendRequest(options: SoapRequestOptions): Promise<any> {
        const soapEnvelope = `<?xmlversion="1.0"encoding="UTF-8"?>
                <soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns="https://siat.impuestos.gob.bo/">
                <soap:Header/>
                <soap:Body>
                    ${options.xmlBody}
                </soap:Body>
                </soap:Envelope>`;
 
        try {
            const res = await axios.post(options.url, soapEnvelope, {
                headers: {
                    'Content-Type': 'text/xml;charset=UTF-8',
                    'SOAPAction': options.action || '',
                    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)'
                },
                timeout: 15000
            })
            const parser = new xml2js.Parser({ explicitArray: false, ignoreAttrs: true });
            const result = await parser.parserStringPromise(res.data);
            return result;
        }
        catch (error: any) {
            if (error.response) {
                const parser = new xml2js.Parser({ explicitArray: false, ignoreAttrs: true });
                try {
                    const errorData = await parser.parseStringPromise(error.response.data);
                    throw new Error(`Error SOAP SIAT: ${JSON.stringify(errorData)}`);
                } catch {
                    throw new Error(`Error HTTP SIAT (${error.response.status}): ${error.response.statusText}`);
                }
            }
        }
    }
}